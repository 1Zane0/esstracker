//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// melde schickt eine Schreibtisch-Benachrichtigung. Aus dmenu gestartet gibt
// es kein Terminal, in dem stderr zu lesen waere.
func melde(titel, text string) {
	if pfad, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(pfad, "-u", "critical", titel, text).Run()
	}
}

// konsoleLoesen tut hier nichts — siehe plattform_windows.go.
func konsoleLoesen() {}

// ---------- Einrichten ----------

// einrichten legt Starter und Symbol im Anwendungsmenue ab. Fuer dmenu_run
// braucht es das nicht — das liest den PATH —, aber ein Programm ohne Eintrag
// im Menue ist fuer jeden anderen Starter unsichtbar.
func einrichten() error {
	pfad, err := os.Executable()
	if err != nil {
		return err
	}
	heim, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ordner := heim + "/.local/share/applications"
	if err := os.MkdirAll(ordner, 0o755); err != nil {
		return err
	}
	symbolOrdner := heim + "/.local/share/icons/hicolor/scalable/apps"
	if err := os.MkdirAll(symbolOrdner, 0o755); err != nil {
		return err
	}
	symbol := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <rect width="128" height="128" rx="28" fill="#0A0A0B"/>
  <path d="M40 30v30a12 12 0 0 0 24 0V30" stroke="#FAFAFA" stroke-width="7"
        fill="none" stroke-linecap="round"/>
  <path d="M52 30v68" stroke="#FAFAFA" stroke-width="7" stroke-linecap="round"/>
  <path d="M88 30c-8 0-12 10-12 22s4 16 12 16" stroke="#FAFAFA" stroke-width="7"
        fill="none" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M88 30v68" stroke="#FAFAFA" stroke-width="7" stroke-linecap="round"/>
</svg>
`
	if err := os.WriteFile(symbolOrdner+"/esstracker.svg", []byte(symbol), 0o644); err != nil {
		return err
	}
	starter := `[Desktop Entry]
Type=Application
Name=Esstracker
GenericName=Ernährungstagebuch
Comment=Was gegessen wurde, vom Handy auf diesen Rechner
Exec=` + pfad + `
Icon=esstracker
Terminal=false
Categories=Utility;Office;
StartupNotify=true
StartupWMClass=esstracker
`
	if err := os.WriteFile(ordner+"/esstracker.desktop", []byte(starter), 0o644); err != nil {
		return err
	}
	fmt.Println("geschrieben: " + ordner + "/esstracker.desktop")
	fmt.Println("geschrieben: " + symbolOrdner + "/esstracker.svg")
	return nil
}
