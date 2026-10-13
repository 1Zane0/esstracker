package ki

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// falschesClaude legt ein Programm namens claude vorne in den PATH. Es schreibt
// jeden Aufruf mit seinen Argumenten in eine eigene Datei und antwortet mit
// dem, was `claude -p --output-format json --json-schema …` ausgibt.
//
// Das Programm ist dieses Testprogramm selbst, unter dem anderen Namen
// abgelegt: ein Shell-Skript liesse sich unter Windows nicht starten. TestMain
// erkennt an der Umgebung, dass es als claude gerufen wurde.
func falschesClaude(t *testing.T, antwort string) (ordner string) {
	t.Helper()
	ordner = t.TempDir()
	selbst, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	roh, err := os.ReadFile(selbst)
	if err != nil {
		t.Fatal(err)
	}
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(ordner, name), roh, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ordner, "antwort"), []byte(antwort+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(alsClaude, ordner)
	t.Setenv("PATH", ordner+string(os.PathListSeparator)+os.Getenv("PATH"))
	return ordner
}

// alsClaude traegt den Ordner, in dem das falsche claude mitschreibt. Steht
// die Variable, ist dieser Prozess nicht der Test, sondern das falsche claude.
const alsClaude = "ESSTRACKER_TEST_ALS_CLAUDE"

func TestMain(m *testing.M) {
	if ordner := os.Getenv(alsClaude); ordner != "" {
		os.Exit(spieleClaude(ordner))
	}
	os.Exit(m.Run())
}

func spieleClaude(ordner string) int {
	frueher, _ := filepath.Glob(filepath.Join(ordner, "aufruf-*"))
	var argumente bytes.Buffer
	for _, a := range os.Args[1:] {
		argumente.WriteString(a)
		argumente.WriteByte(0)
	}
	ziel := filepath.Join(ordner, fmt.Sprintf("aufruf-%d", len(frueher)))
	if err := os.WriteFile(ziel, argumente.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	antwort, err := os.ReadFile(filepath.Join(ordner, "antwort"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, _ = os.Stdout.Write(antwort)
	return 0
}

func aufrufe(t *testing.T, ordner string) [][]string {
	t.Helper()
	dateien, _ := filepath.Glob(filepath.Join(ordner, "aufruf-*"))
	var alle [][]string
	for _, d := range dateien {
		roh, err := os.ReadFile(d)
		if err != nil {
			t.Fatal(err)
		}
		teile := strings.Split(string(bytes.TrimSuffix(roh, []byte{0})), "\x00")
		alle = append(alle, teile)
	}
	return alle
}

func warteBisFertig(t *testing.T, s *Schaetzer) {
	t.Helper()
	ende := time.Now().Add(10 * time.Second)
	for time.Now().Before(ende) {
		if !s.Stand().Laeuft {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("der Schaetzer wurde nicht fertig")
}

const antwortVierFaelle = `{"type":"result","subtype":"success","is_error":false,"result":"","modelUsage":{"claude-haiku-4-5-20251001":{"outputTokens":180}},"structured_output":{"eintraege":[{"id":"o1","name":"Kulturheidelbeeren","kcal":57,"fett":0.3,"gesaettigt":0,"kohlenhydrate":14.5,"zucker":10,"ballaststoffe":2.4,"eiweiss":0.7,"salz":0},{"id":"o2","name":"Wasser","kcal":5000,"fett":0,"gesaettigt":0,"kohlenhydrate":0,"zucker":0,"ballaststoffe":0,"eiweiss":0,"salz":0,"sicherheit":"hoch","begruendung":"falsch"},{"id":"fremd","name":"x","kcal":100,"fett":0,"gesaettigt":0,"kohlenhydrate":0,"zucker":0,"ballaststoffe":0,"eiweiss":0,"salz":0,"sicherheit":"hoch","begruendung":""}],"unklar":[{"id":"o3","grund":"Zeug kann alles sein."}]}}`

func TestSchaetzerTraegtEinUndLaesstUnfugOffen(t *testing.T) {
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	ordner := falschesClaude(t, antwortVierFaelle)

	b, err := tagebuch.Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"o1", "o2", "o3", "o4"} {
		e := tagebuch.Eintrag{
			ID: id, Name: "kulturheidelbeeren", Menge: 150, Quelle: tagebuch.QuelleOffen,
			Zeit: time.Now().Format(time.RFC3339),
		}
		if _, err := b.Aufnehmen(e, "handy"); err != nil {
			t.Fatal(err)
		}
	}

	s := NeuerSchaetzer(b)
	s.Anstossen(false)
	warteBisFertig(t, s)

	quelle := map[string]string{}
	for _, e := range b.Sicht().Tage[0].Zeilen {
		quelle[e.ID] = e.Quelle
		if e.ID == "o1" && (e.Kcal != 86 || e.Schaetzung == nil || e.Schaetzung.Modell != "claude-haiku-4-5-20251001") {
			t.Errorf("o1: Kcal %d (erwartet 86), Schaetzung %+v", e.Kcal, e.Schaetzung)
		}
	}
	if quelle["o1"] != tagebuch.QuelleKI {
		t.Errorf("o1 ist %q, erwartet ki", quelle["o1"])
	}
	for _, id := range []string{"o2", "o3", "o4"} {
		if quelle[id] != tagebuch.QuelleOffen {
			t.Errorf("%s ist %q, erwartet offen", id, quelle[id])
		}
	}

	st := s.Stand()
	if st.Fehler != "" {
		t.Errorf("Fehler = %q", st.Fehler)
	}
	if !strings.Contains(st.Unklar["o2"], "5000") {
		t.Errorf("o2: Grund %q sollte die unplausible Zahl nennen", st.Unklar["o2"])
	}
	if st.Unklar["o3"] != "Zeug kann alles sein." {
		t.Errorf("o3: Grund %q", st.Unklar["o3"])
	}
	if st.Unklar["o4"] == "" {
		t.Error("o4: kein Grund, obwohl Claude ihn ausgelassen hat")
	}

	// Die Schalter, auf die es ankommt.
	liste := aufrufe(t, ordner)
	if len(liste) != 1 {
		t.Fatalf("%d Aufrufe, erwartet 1", len(liste))
	}
	args := liste[0]
	nach := func(schalter string) string {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == schalter {
				return args[i+1]
			}
		}
		return "(fehlt)"
	}
	if nach("--model") != "haiku" {
		t.Errorf("--model = %q, erwartet haiku", nach("--model"))
	}
	if nach("--effort") != Aufwand {
		t.Errorf("--effort = %q, erwartet %s", nach("--effort"), Aufwand)
	}
	if nach("--tools") != "" {
		t.Errorf("--tools = %q, erwartet leer", nach("--tools"))
	}
	if args[len(args)-2] != schema {
		t.Error("das Schema steht nicht direkt vor dem Text — der Text koennte als Wert von --tools enden")
	}
	if text := args[len(args)-1]; !strings.Contains(text, `"id": "o4"`) || !strings.Contains(text, "<bekannt>") {
		t.Errorf("der Text traegt die offenen Eintraege nicht:\n%s", text)
	}

	// Von allein wird nicht noch einmal versucht, was eben nichts ergab …
	s.Anstossen(false)
	warteBisFertig(t, s)
	if n := len(aufrufe(t, ordner)); n != 1 {
		t.Errorf("nach erneutem Anstossen von allein %d Aufrufe, erwartet weiter 1", n)
	}
	// … von Hand schon.
	s.Anstossen(true)
	warteBisFertig(t, s)
	if n := len(aufrufe(t, ordner)); n != 2 {
		t.Errorf("nach Anstossen von Hand %d Aufrufe, erwartet 2", n)
	}
}

func TestSchaetzerMeldetFehlerDerCLI(t *testing.T) {
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	falschesClaude(t, `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"Not logged in"}`)

	b, err := tagebuch.Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	e := tagebuch.Eintrag{ID: "o1", Name: "Döner", Menge: 350, Quelle: tagebuch.QuelleOffen, Zeit: time.Now().Format(time.RFC3339)}
	if _, err := b.Aufnehmen(e, "handy"); err != nil {
		t.Fatal(err)
	}

	s := NeuerSchaetzer(b)
	s.Anstossen(false)
	warteBisFertig(t, s)

	if st := s.Stand(); !strings.Contains(st.Fehler, "Not logged in") {
		t.Errorf("Fehler = %q, erwartet die Meldung der CLI", st.Fehler)
	}
	if len(b.Offene()) != 1 {
		t.Error("der Eintrag ist trotz Fehler nicht mehr offen")
	}
}

func TestVorschlagPruefe(t *testing.T) {
	faelle := []struct {
		name  string
		v     vorschlag
		taugt bool
	}{
		{"Heidelbeeren", vorschlag{Kcal: 57, Kohlenhydrate: 14.5, Zucker: 10}, true},
		{"reines Oel", vorschlag{Kcal: 884, Fett: 100}, true},
		{"null kcal", vorschlag{Kcal: 0}, false},
		{"mehr als Fett", vorschlag{Kcal: 950, Fett: 100}, false},
		{"negativ", vorschlag{Kcal: 100, Fett: -1}, false},
		{"zu viel Masse", vorschlag{Kcal: 400, Fett: 50, Kohlenhydrate: 60}, false},
	}
	for _, f := range faelle {
		_, warum := f.v.pruefe()
		if (warum == "") != f.taugt {
			t.Errorf("%s: warum = %q, taugt erwartet %v", f.name, warum, f.taugt)
		}
	}

	// Zucker ueber den Kohlenhydraten ist Rundung, kein Grund zum Verwerfen.
	n, warum := vorschlag{Kcal: 300, Kohlenhydrate: 50, Zucker: 52, Fett: 3, Gesaettigt: 4}.pruefe()
	if warum != "" || n.Zucker != 50 || n.Gesaettigt != 3 {
		t.Errorf("gekappt: %+v, warum %q", n, warum)
	}
}
