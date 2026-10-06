package tagebuch

import (
	"strings"
	"testing"
	"time"
)

func frischesBuch(t *testing.T) *Buch {
	t.Helper()
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	b, err := Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEinkaufSchreibenErsetztUndErgaenzt(t *testing.T) {
	b := frischesBuch(t)

	erste := []Posten{
		{Name: "Magerquark", Menge: 500, Einheit: "g", Gruppe: "Kühlregal", Grund: "3× diese Woche"},
		{Name: "Bananen", Menge: 6, Einheit: "Stück", Gruppe: "Obst & Gemüse"},
		{Name: "", Menge: 1}, // ohne Namen: faellt weg, ohne die anderen mitzunehmen
	}
	if err := b.EinkaufSchreiben(erste, true, "für drei Tage", "Zwei Posten.", "haiku"); err != nil {
		t.Fatal(err)
	}
	e := b.EinkaufHolen()
	if len(e.Posten) != 2 {
		t.Fatalf("%d Posten, erwartet 2: %+v", len(e.Posten), e.Posten)
	}
	if e.Wunsch != "für drei Tage" || e.Modell != "haiku" || e.Notiz != "Zwei Posten." {
		t.Errorf("Kopf der Liste stimmt nicht: %+v", e)
	}
	// Nach Abteilung geordnet: Obst kommt im Laden vor dem Kuehlregal.
	if e.Posten[0].Name != "Bananen" {
		t.Errorf("nicht nach Abteilung sortiert: %s zuerst", e.Posten[0].Name)
	}
	for _, p := range e.Posten {
		if p.Von != VonKI || p.ID == "" || p.Angelegt == "" {
			t.Errorf("Posten unvollstaendig: %+v", p)
		}
	}

	// Ergaenzen laesst die Liste stehen und wirft Doppeltes weg — auch in
	// anderer Schreibweise.
	zweite := []Posten{
		{Name: "bananen", Menge: 3, Einheit: "Stück", Gruppe: "Obst & Gemüse"},
		{Name: "Haferflocken", Menge: 1, Einheit: "Packung", Gruppe: "Trockenwaren"},
	}
	if err := b.EinkaufSchreiben(zweite, false, "Frühstück", "", "haiku"); err != nil {
		t.Fatal(err)
	}
	e = b.EinkaufHolen()
	if len(e.Posten) != 3 {
		t.Fatalf("%d Posten nach dem Ergaenzen, erwartet 3: %+v", len(e.Posten), e.Posten)
	}
	for _, p := range e.Posten {
		if strings.EqualFold(p.Name, "bananen") && p.Menge != 6 {
			t.Errorf("die bestehende Zeile wurde ueberschrieben: %+v", p)
		}
	}

	// Nichts Neues ist eine Meldung wert: sonst sieht es aus, als waere der
	// Aufruf ins Leere gelaufen.
	if err := b.EinkaufSchreiben(zweite, false, "", "", "haiku"); err == nil {
		t.Error("kein Fehler, obwohl alles schon draufstand")
	}
}

func TestPostenSchrittUndMenge(t *testing.T) {
	b := frischesBuch(t)
	if err := b.EinkaufSchreiben([]Posten{
		{Name: "Mehl", Menge: 1000, Einheit: "g", Gruppe: "Trockenwaren"},
		{Name: "Eier", Menge: 6, Einheit: "Stück", Gruppe: "Kühlregal"},
	}, true, "", "", "haiku"); err != nil {
		t.Fatal(err)
	}
	nach := func(name string) Posten {
		t.Helper()
		for _, p := range b.EinkaufHolen().Posten {
			if p.Name == name {
				return p
			}
		}
		t.Fatalf("%s ist weg", name)
		return Posten{}
	}

	mehl := nach("Mehl")
	if _, err := b.PostenSchritt(mehl.ID, 1, ""); err != nil {
		t.Fatal(err)
	}
	if got := nach("Mehl").Menge; got != 1050 {
		t.Errorf("Mehl nach + = %v, erwartet 1050 (Schritt 50 g)", got)
	}

	eier := nach("Eier")
	if _, err := b.PostenSchritt(eier.ID, -1, ""); err != nil {
		t.Fatal(err)
	}
	if got := nach("Eier").Menge; got != 5 {
		t.Errorf("Eier nach − = %v, erwartet 5", got)
	}

	// Nach unten endet es bei einem Schritt: ein Posten mit der Menge null
	// stuende auf der Liste und behauptete gleichzeitig, es sei nichts zu holen.
	for i := 0; i < 10; i++ {
		if _, err := b.PostenSchritt(eier.ID, -1, ""); err != nil {
			t.Fatal(err)
		}
	}
	if got := nach("Eier").Menge; got != 1 {
		t.Errorf("Eier nach zehnmal − = %v, erwartet 1", got)
	}

	// Eingetippte Menge samt getauschter Einheit.
	if err := b.PostenMenge(mehl.ID, 2, "kg", "", ""); err != nil {
		t.Fatal(err)
	}
	if p := nach("Mehl"); p.Menge != 2 || p.Einheit != "kg" || p.MengeText() != "2 kg" {
		t.Errorf("Mehl = %+v, MengeText %q", p, p.MengeText())
	}
	// Unfug bleibt draussen.
	if err := b.PostenMenge(mehl.ID, -3, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if got := nach("Mehl").Menge; got != 2 {
		t.Errorf("negative Menge wurde uebernommen: %v", got)
	}
}

func TestPostenHakenAelteresSchlaegtNichtJuengeres(t *testing.T) {
	b := frischesBuch(t)
	if err := b.EinkaufSchreiben([]Posten{{Name: "Butter", Menge: 1, Einheit: "Packung", Gruppe: "Kühlregal"}},
		true, "", "", "haiku"); err != nil {
		t.Fatal(err)
	}
	id := b.EinkaufHolen().Posten[0].ID

	spaet := time.Now().Format(time.RFC3339)
	frueh := time.Now().Add(-time.Hour).Format(time.RFC3339)

	if err := b.PostenHaken(id, true, spaet, "handy"); err != nil {
		t.Fatal(err)
	}
	// Der Nachzuegler von vor einer Stunde darf den Haken nicht zurueckdrehen.
	if err := b.PostenHaken(id, false, frueh, "handy"); err != nil {
		t.Fatal(err)
	}
	if p := b.EinkaufHolen().Posten[0]; !p.Erledigt {
		t.Errorf("ein aelterer Haken hat den juengeren ueberschrieben: %+v", p)
	}

	// Ein Posten, der dann wirklich wieder offen soll, geht weiter.
	if err := b.PostenHaken(id, false, time.Now().Add(time.Minute).Format(time.RFC3339), ""); err != nil {
		t.Fatal(err)
	}
	if b.EinkaufHolen().Posten[0].Erledigt {
		t.Error("der Haken liess sich nicht mehr loesen")
	}
}

func TestPostenVomHandyNurEinmal(t *testing.T) {
	b := frischesBuch(t)
	vomHandy := []Posten{{ID: "h-1", Name: "Klopapier", Menge: 1, Einheit: "Packung", Gruppe: "Haushalt"}}

	for i := 0; i < 3; i++ {
		neu, err := b.PostenAufnehmen(vomHandy, "handy-a")
		if err != nil {
			t.Fatal(err)
		}
		if (i == 0) != (neu == 1) {
			t.Errorf("Durchgang %d: neu = %d", i, neu)
		}
	}
	e := b.EinkaufHolen()
	if len(e.Posten) != 1 || e.Posten[0].Wer != "handy-a" || e.Posten[0].Von != VonHand {
		t.Errorf("Liste nach dreimal Nachschicken: %+v", e.Posten)
	}
}

func TestPostenAnlegenUndAufraeumen(t *testing.T) {
	b := frischesBuch(t)

	p, err := b.PostenAnlegen(Posten{Name: "  Tomaten  ", Gruppe: "Erfundene Abteilung", Einheit: "Handvoll"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Tomaten" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Gruppe != GruppeSonstiges {
		t.Errorf("Gruppe = %q, erwartet %s", p.Gruppe, GruppeSonstiges)
	}
	// Ohne Menge und mit unbekannter Einheit: ein Stueck.
	if p.Einheit != "stueck" || p.Menge != 1 {
		t.Errorf("Menge = %v %s, erwartet 1 stueck", p.Menge, p.Einheit)
	}
	if _, err := b.PostenAnlegen(Posten{Name: "tomaten"}); err == nil {
		t.Error("derselbe Posten liess sich zweimal anlegen")
	}

	if err := b.PostenHaken(p.ID, true, "", ""); err != nil {
		t.Fatal(err)
	}
	weg, err := b.EinkaufAufraeumen()
	if err != nil || weg != 1 {
		t.Fatalf("aufgeraeumt: %d, Fehler %v", weg, err)
	}
	if len(b.EinkaufHolen().Posten) != 0 {
		t.Error("die abgehakte Zeile liegt noch da")
	}
}

func TestAbgleichTraegtDieListe(t *testing.T) {
	b := frischesBuch(t)
	if err := b.EinkaufSchreiben([]Posten{{Name: "Hafermilch", Menge: 2, Einheit: "Packung", Gruppe: "Getränke"}},
		true, "", "", "haiku"); err != nil {
		t.Fatal(err)
	}
	a := b.Abgleichen()
	if len(a.Einkauf.Posten) != 1 || a.Einkauf.Posten[0].Name != "Hafermilch" {
		t.Errorf("der Abgleich traegt die Liste nicht: %+v", a.Einkauf)
	}
	if a.Einkauf.Offen() != 1 {
		t.Errorf("Offen() = %d", a.Einkauf.Offen())
	}
	if a.Einkauf.Stand == "" {
		t.Error("ohne Stand kann das Handy nicht erkennen, ob sich etwas geaendert hat")
	}
}
