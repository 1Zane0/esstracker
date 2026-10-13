//go:build !ohne_fenster

package fenster

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

// Verfuegbar meldet, ob dieses Binary ein Fenster oeffnen kann.
const Verfuegbar = true

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	setProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	getDpiForSystem               = user32.NewProc("GetDpiForSystem")
	getSystemMetrics              = user32.NewProc("GetSystemMetrics")
	showWindow                    = user32.NewProc("ShowWindow")
	setForegroundWindow           = user32.NewProc("SetForegroundWindow")
	dwmSetWindowAttribute         = dwmapi.NewProc("DwmSetWindowAttribute")
)

// Oeffne zeigt das Fenster auf der vollstaendigen Adresse und kehrt erst
// zurueck, wenn es geschlossen wird. Der Aufruf muss aus der Haupt-Goroutine
// kommen: die Nachrichtenschleife gehoert dem Thread, der das Fenster anlegt.
//
// Gezeichnet wird mit WebView2, der Laufzeit von Edge, die Windows 10 und 11
// mitbringen. Die Bindung ist reines Go — kein C-Compiler, kein cgo.
func Oeffne(url, titel string) error {
	runtime.LockOSThread()

	// Ohne diesen Aufruf haelt Windows das Programm fuer eines, das von
	// Skalierung nichts weiss, und zieht das fertige Bild auf 125 oder 150 %
	// hoch — die Schrift wird weich. -4 ist PER_MONITOR_AWARE_V2.
	_, _, _ = setProcessDpiAwarenessContext.Call(^uintptr(3))

	// Dunkel ist keine Vorliebe, sondern Teil des Entwurfs. Ohne diesen Wert
	// (--grund aus stil.css) steht das Fenster weiss da, bis die Seite laedt.
	_ = os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", "FF0A0A0B")

	// Der Zwischenspeicher von WebView2 gehoert nicht in den Datenordner:
	// dort liest Claude mit, und ein Backup soll ihn nicht mitnehmen.
	ablage, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("kein Ordner für den Zwischenspeicher des Fensters: %w", err)
	}

	// Wie die Tasklist nebenan: eine Liste braucht keine Breite, und das
	// Jahresraster ist mit 1120 Punkten noch vollstaendig zu sehen. Nach oben
	// begrenzt der Bildschirm — bei 150 % passen 820 Punkte nicht auf 1080.
	breite, hoehe := masse()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  filepath.Join(ablage, "esstracker", "WebView2"),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  titel,
			Width:  uint(breite),
			Height: uint(hoehe),
			IconId: symbolNr,
			Center: true,
		},
	})
	if w == nil {
		return errors.New("das Fenster ließ sich nicht öffnen — fehlt die WebView2-Laufzeit von Microsoft Edge? " +
			"Bis sie da ist: `esstracker dienst` und " + url + " im Browser aufrufen")
	}
	defer w.Destroy()
	griff := uintptr(w.Window())

	// Der Eintrag im Startmenue startet das Programm verkleinert, damit die
	// Konsole nicht aufblitzt (siehe einrichten). Windows gibt diesen Wunsch
	// an das erste Fenster weiter — also wird es ausdruecklich hervorgeholt.
	const swShowNormal = 1
	_, _, _ = showWindow.Call(griff, swShowNormal)
	_, _, _ = setForegroundWindow.Call(griff)

	// Dasselbe wie GTK_THEME unter Linux: sonst sitzt eine helle Titelleiste
	// auf einer dunklen Seite. 20 ist DWMWA_USE_IMMERSIVE_DARK_MODE.
	dunkel := int32(1)
	_, _, _ = dwmSetWindowAttribute.Call(griff, 20, uintptr(unsafe.Pointer(&dunkel)), unsafe.Sizeof(dunkel))

	// Erst nach dem Wink: SetSize zeichnet den Rahmen neu, und erst dabei
	// wird die Titelleiste dunkel.
	w.SetSize(breite, hoehe, webview2.HintNone)
	w.Navigate(url)
	w.Run()
	return nil
}

// symbolNr ist die Nummer des Symbols in den Ressourcen der exe (siehe
// cmd/esstracker/symbol_windows_amd64.syso).
const symbolNr = 1

// masse rechnet die Fenstergroesse in Bildpunkte dieses Bildschirms um und
// laesst Platz fuer Rahmen und Taskleiste.
func masse() (breite, hoehe int) {
	const (
		smCxScreen = 0
		smCyScreen = 1
	)
	dpi, _, _ := getDpiForSystem.Call()
	if dpi == 0 {
		dpi = 96
	}
	schirmB, _, _ := getSystemMetrics.Call(smCxScreen)
	schirmH, _, _ := getSystemMetrics.Call(smCyScreen)

	punkte := func(n int) int { return n * int(dpi) / 96 }
	breite = min(punkte(1120), int(schirmB)-punkte(40))
	hoehe = min(punkte(820), int(schirmH)-punkte(110))
	return breite, hoehe
}
