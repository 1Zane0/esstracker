// Package speicher haelt das, was ueber Neustarts hinweg bleiben muss — als
// eine JSON-Datei im Datenverzeichnis des Benutzers.
//
// Warum keine Datenbank: ein Essenseintrag ist knapp ein Kilobyte, und selbst
// wer jeden Tag ein Dutzend eintraegt, kommt im Jahr auf wenige Megabyte. Eine
// SQLite-Bindung waere eine Abhaengigkeit mehr und ein Grund mehr, warum das
// Werkzeug nach einem Update nicht startet. Der Preis: alles liegt beim
// Schreiben komplett im Speicher — bei dieser Groesse in Ordnung, und hier
// benannt, falls es das mal nicht mehr ist. Ab etwa hunderttausend Eintraegen
// waere der Punkt erreicht, an dem das neu zu entscheiden ist.
package speicher

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Ordner ist das Verzeichnis, in dem die Datei liegt. Es folgt der
// XDG-Konvention, damit ein Backup des Home-Verzeichnisses sie mitnimmt.
func Ordner() string {
	if p := os.Getenv("ESSTRACKER_DATEN"); p != "" {
		return p
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "esstracker")
	}
	heim, err := os.UserHomeDir()
	if err != nil {
		return ".esstracker"
	}
	return filepath.Join(heim, ".local", "share", "esstracker")
}

// Pfad liefert den vollen Pfad einer Datei im Datenverzeichnis.
func Pfad(name string) string { return filepath.Join(Ordner(), name) }

var schreibsperre sync.Mutex

// Lies laedt eine JSON-Datei in ziel. Fehlt die Datei, bleibt ziel unberuehrt
// und der Rueckgabewert ist nil — "noch nichts da" ist kein Fehler, sondern
// der Normalfall beim ersten Start.
func Lies(name string, ziel any) error {
	roh, err := os.ReadFile(Pfad(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s lesen: %w", name, err)
	}
	if len(roh) == 0 {
		return nil
	}
	if err := json.Unmarshal(roh, ziel); err != nil {
		return fmt.Errorf("%s auswerten: %w", name, err)
	}
	return nil
}

// Schreib legt wert als JSON ab — erst in eine Nachbardatei, dann per Rename
// an den Zielort. Ein Stromausfall mitten im Schreiben hinterlaesst so die
// alte, vollstaendige Datei statt einer halben neuen.
func Schreib(name string, wert any) error {
	schreibsperre.Lock()
	defer schreibsperre.Unlock()

	if err := os.MkdirAll(Ordner(), 0o700); err != nil {
		return fmt.Errorf("datenordner anlegen: %w", err)
	}
	roh, err := json.MarshalIndent(wert, "", "  ")
	if err != nil {
		return fmt.Errorf("%s kodieren: %w", name, err)
	}
	ziel := Pfad(name)
	temp := ziel + ".neu"
	if err := os.WriteFile(temp, roh, 0o600); err != nil {
		return fmt.Errorf("%s schreiben: %w", name, err)
	}
	if err := os.Rename(temp, ziel); err != nil {
		return fmt.Errorf("%s ersetzen: %w", name, err)
	}
	return nil
}
