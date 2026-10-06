package ki

import (
	"strings"
	"testing"
	"time"

	"github.com/1Zane0/esstracker/internal/tagebuch"
)

const antwortDreiPosten = `{"type":"result","subtype":"success","is_error":false,"result":"","modelUsage":{"claude-haiku-4-5-20251001":{"outputTokens":220}},"structured_output":{"posten":[{"name":"Magerquark","menge":500,"einheit":"g","gruppe":"Kühlregal","grund":"4× diese Woche gegessen"},{"name":"Bananen","menge":6,"einheit":"Stück","gruppe":"Obst & Gemüse","grund":"steht oft im Tagebuch"},{"name":"Haferflocken","menge":1,"einheit":"Packung","gruppe":"Trockenwaren","grund":"für das Frühstück"}],"notiz":"Drei Posten nach dem, was zuletzt auf dem Teller lag."}}`

func warteBisPlanerFertig(t *testing.T, p *Planer) {
	t.Helper()
	ende := time.Now().Add(10 * time.Second)
	for time.Now().Before(ende) {
		if !p.Stand().Laeuft {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("der Planer wurde nicht fertig")
}

func TestPlanerSchreibtDieListe(t *testing.T) {
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	ordner := falschesClaude(t, antwortDreiPosten)

	b, err := tagebuch.Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	// Ein Eintrag im Tagebuch, damit der Stand nicht leer ist.
	if _, err := b.Aufnehmen(tagebuch.Eintrag{
		ID: "e1", Name: "Magerquark", Menge: 250, Quelle: tagebuch.QuelleOFF,
		Je100: tagebuch.Naehrwerte{Kcal: 67}, Zeit: time.Now().Format(time.RFC3339),
	}, "handy"); err != nil {
		t.Fatal(err)
	}
	// Ein Posten von Hand, der beim Ersetzen weichen muss.
	if _, err := b.PostenAnlegen(tagebuch.Posten{Name: "Altes Zeug"}); err != nil {
		t.Fatal(err)
	}

	p := NeuerPlaner(b)
	if err := p.Erstellen("für drei Tage, viel Eiweiß", false); err != nil {
		t.Fatal(err)
	}
	warteBisPlanerFertig(t, p)

	if st := p.Stand(); st.Fehler != "" {
		t.Fatalf("Fehler = %q", st.Fehler)
	}
	e := b.EinkaufHolen()
	if len(e.Posten) != 3 {
		t.Fatalf("%d Posten, erwartet 3: %+v", len(e.Posten), e.Posten)
	}
	if e.Notiz == "" || e.Wunsch != "für drei Tage, viel Eiweiß" {
		t.Errorf("Kopf der Liste: %+v", e)
	}
	if e.Modell != "claude-haiku-4-5-20251001" {
		t.Errorf("Modell = %q", e.Modell)
	}
	for _, p := range e.Posten {
		if p.Von != tagebuch.VonKI {
			t.Errorf("%s kommt als %q", p.Name, p.Von)
		}
		if p.Name == "Magerquark" && (p.Menge != 500 || p.Einheit != "g") {
			t.Errorf("Magerquark = %v %s", p.Menge, p.Einheit)
		}
		if p.Name == "Altes Zeug" {
			t.Error("die alte Liste wurde nicht ersetzt")
		}
	}

	// Die Schalter, auf die es ankommt — dieselben wie beim Schaetzen.
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
	if nach("--model") != Modell || nach("--effort") != Aufwand {
		t.Errorf("--model %q, --effort %q", nach("--model"), nach("--effort"))
	}
	if nach("--tools") != "" {
		t.Errorf("--tools = %q, erwartet leer", nach("--tools"))
	}
	if args[len(args)-2] != einkaufSchema {
		t.Error("das Schema steht nicht direkt vor dem Text")
	}
	text := args[len(args)-1]
	for _, muss := range []string{"<stand", "<liste>", "Altes Zeug", "Wunsch: für drei Tage, viel Eiweiß", `art="neu"`} {
		if !strings.Contains(text, muss) {
			t.Errorf("im Text fehlt %q:\n%s", muss, kuerzen(text, 600))
		}
	}
	// Die Einheiten im Schema kommen aus dem Tagebuch und nicht aus einer
	// zweiten Liste hier.
	if !strings.Contains(einkaufSchema, "Packung") || !strings.Contains(einkaufSchema, "Kühlregal") {
		t.Error("das Schema kennt die Einheiten oder Abteilungen des Tagebuchs nicht")
	}
}

func TestPlanerErgaenztOhneDieListeWegzuwerfen(t *testing.T) {
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	falschesClaude(t, antwortDreiPosten)

	b, err := tagebuch.Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.PostenAnlegen(tagebuch.Posten{Name: "Klopapier", Gruppe: "Haushalt"}); err != nil {
		t.Fatal(err)
	}

	p := NeuerPlaner(b)
	if err := p.Erstellen("", true); err != nil {
		t.Fatal(err)
	}
	warteBisPlanerFertig(t, p)

	e := b.EinkaufHolen()
	if len(e.Posten) != 4 {
		t.Fatalf("%d Posten, erwartet 4 (drei neue und Klopapier): %+v", len(e.Posten), e.Posten)
	}
}

func TestPlanerMeldetFehlerDerCLI(t *testing.T) {
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	falschesClaude(t, `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"Not logged in"}`)

	b, err := tagebuch.Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	p := NeuerPlaner(b)
	if err := p.Erstellen("egal", false); err != nil {
		t.Fatal(err)
	}
	warteBisPlanerFertig(t, p)

	if st := p.Stand(); !strings.Contains(st.Fehler, "Not logged in") {
		t.Errorf("Fehler = %q, erwartet die Meldung der CLI", st.Fehler)
	}
	if len(b.EinkaufHolen().Posten) != 0 {
		t.Error("trotz Fehler steht etwas auf der Liste")
	}
}

func TestPlanerVerwirftLeereAntwort(t *testing.T) {
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	falschesClaude(t, `{"type":"result","subtype":"success","is_error":false,"result":"","structured_output":{"posten":[],"notiz":"nichts"}}`)

	b, err := tagebuch.Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	p := NeuerPlaner(b)
	if err := p.Erstellen("", false); err != nil {
		t.Fatal(err)
	}
	warteBisPlanerFertig(t, p)

	if st := p.Stand(); st.Fehler == "" {
		t.Error("kein Fehler, obwohl kein Posten kam")
	}
}
