//go:build !ohne_fenster && !windows

// Package fenster oeffnet das Programmfenster.
//
// Warum WebKitGTK und nicht GTK-Widgets: die Oberflaeche ist ein Raster aus
// vielen kleinen Feldern, das sich in HTML in ein paar Zeilen beschreiben
// laesst und in Widgets in ein paar hundert. Das Fenster ist ein echtes
// Fenster mit eigenem Eintrag in der Taskleiste — nur die Zeichenflaeche
// darin ist HTML, und die kommt aus dem eigenen Binary, nicht aus dem Netz.
//
// Mit `-tags ohne_fenster` faellt dieser Weg weg; dann bleibt der Dienst, der
// ohne Bildschirm laeuft.
//
// Unter Windows steht dasselbe Fenster auf WebView2 — siehe fenster_windows.go.
package fenster

import (
	"fmt"
	"os"
	"runtime"

	webview "github.com/webview/webview_go"
)

// Verfuegbar meldet, ob dieses Binary ein Fenster oeffnen kann.
const Verfuegbar = true

// Oeffne zeigt das Fenster auf der vollstaendigen Adresse und kehrt erst
// zurueck, wenn es geschlossen wird. Der Aufruf muss aus der Haupt-Goroutine
// kommen: GTK duldet seine Ereignisschleife nur auf dem Thread, auf dem das
// Programm gestartet ist.
func Oeffne(url, titel string) error {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("kein Bildschirm gefunden (weder DISPLAY noch WAYLAND_DISPLAY).\n"+
			"Ohne grafische Sitzung gibt es kein Fenster — nimm `esstracker dienst` und ruf %s im Browser auf.", url)
	}
	// Dunkel ist keine Vorliebe, sondern Teil des Entwurfs: die Oberflaeche
	// hat nur eine Fassung. Ohne diesen Wink zeichnet GTK Rahmen und
	// Bildlaufleisten hell um eine dunkle Seite herum.
	_ = os.Setenv("GTK_THEME", "Adwaita:dark")

	runtime.LockOSThread()

	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle(titel)
	// Wie die Tasklist nebenan: eine Liste braucht keine Breite, und das
	// Jahresraster ist mit 1120 Punkten noch vollstaendig zu sehen.
	w.SetSize(1120, 820, webview.HintNone)
	w.Navigate(url)
	w.Run()
	return nil
}
