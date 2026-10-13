package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")

	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	getConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	freeConsole           = kernel32.NewProc("FreeConsole")
	messageBox            = user32.NewProc("MessageBoxW")
)

// melde zeigt die Meldung in einem Dialog — aber nur, wenn es kein Terminal
// gibt, in dem stderr zu lesen waere. Dort stuende der Dialog bloss im Weg.
func melde(titel, text string) {
	if fenster, _, _ := getConsoleWindow.Call(); fenster != 0 {
		return
	}
	const mbIconError = 0x10
	t, _ := windows.UTF16PtrFromString(text)
	k, _ := windows.UTF16PtrFromString(titel)
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(k)), mbIconError)
}

// konsoleLoesen gibt die Konsole zurueck, die Windows dem Programm nur fuer
// sich aufgemacht hat.
//
// Das Programm ist mit Absicht eines fuer die Konsole: nur so warten cmd und
// PowerShell auf `esstracker heute`. Der Preis ist ein schwarzes Fenster beim
// Start aus dem Startmenue. Haengt ausser diesem Prozess niemand daran, hat
// es keiner bestellt, und es geht wieder zu.
func konsoleLoesen() {
	var kennungen [2]uint32
	zahl, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&kennungen[0])), uintptr(len(kennungen)))
	if zahl == 1 {
		_, _, _ = freeConsole.Call()
	}
}

// ---------- Einrichten ----------

// einrichten legt einen Eintrag im Startmenue an — das, was unter Linux die
// .desktop-Datei ist. Das Symbol steckt in der exe und braucht keine eigene
// Datei.
//
// WindowStyle 7 heisst verkleinert: so erscheint die Konsole, die Windows dem
// Programm beim Start mitgibt, nur als Knopf in der Taskleiste und nicht als
// schwarzes Fenster, bis konsoleLoesen sie zurueckgibt.
//
// Eine Verknuepfung ist ein Binaerformat; geschrieben wird sie von der
// PowerShell, die Windows mitbringt. Die Pfade gehen ueber die Umgebung, damit
// im Skript nichts zu maskieren ist.
func einrichten() error {
	pfad, err := os.Executable()
	if err != nil {
		return err
	}
	ziel := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Esstracker.lnk")

	const skript = `$ErrorActionPreference = 'Stop'; ` +
		`$v = (New-Object -ComObject WScript.Shell).CreateShortcut($env:ESSTRACKER_VERKNUEPFUNG); ` +
		`$v.TargetPath = $env:ESSTRACKER_PROGRAMM; ` +
		`$v.WorkingDirectory = Split-Path $env:ESSTRACKER_PROGRAMM; ` +
		`$v.Description = 'Was gegessen wurde, vom Handy auf diesen Rechner'; ` +
		`$v.WindowStyle = 7; ` +
		`$v.Save()`
	befehl := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", skript)
	befehl.Env = append(os.Environ(), "ESSTRACKER_VERKNUEPFUNG="+ziel, "ESSTRACKER_PROGRAMM="+pfad)
	if ausgabe, err := befehl.CombinedOutput(); err != nil {
		return fmt.Errorf("verknüpfung anlegen: %w\n%s", err, strings.TrimSpace(string(ausgabe)))
	}
	fmt.Println("geschrieben: " + ziel)
	return nil
}
