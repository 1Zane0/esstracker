package ki

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// claudePfad sucht die CLI im PATH — und zwar das Programm selbst, nicht die
// Stapeldatei davor.
//
// npm legt unter Windows nur claude.cmd in den PATH. Durch cmd.exe kommen die
// Argumente hier nicht heil an: das Schema ist voller Anfuehrungszeichen, und
// der Text hoert fuer cmd.exe am ersten Zeilenumbruch auf. Das Paket bringt
// aber das eigentliche Programm mit (sein bin-Eintrag ist bin/claude.exe), und
// das liegt von der Stapeldatei aus immer an derselben Stelle.
func claudePfad() (string, error) {
	if pfad, err := exec.LookPath("claude.exe"); err == nil {
		return pfad, nil
	}
	huelle, err := exec.LookPath("claude")
	if err != nil {
		return "", err
	}
	pfad := filepath.Join(filepath.Dir(huelle), "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
	if _, err := os.Stat(pfad); err != nil {
		return "", err
	}
	return pfad, nil
}

// ohneKonsole verhindert, dass Windows fuer jeden Aufruf ein Konsolenfenster
// aufmacht. Das Fenster des Esstrackers hat keine Konsole, die ein
// Kindprozess erben koennte — ohne den Schalter blitzt bei jeder Schaetzung
// ein schwarzes Fenster auf.
func ohneKonsole(befehl *exec.Cmd) {
	const createNoWindow = 0x08000000
	befehl.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
