package tagebuch

import (
	"math"
	"testing"
	"time"
)

// Die Tests schreiben in ein eigenes Verzeichnis je Lauf: das Buch sichert
// sich bei jeder Aenderung auf die Platte, und das darf nicht das echte
// Tagebuch treffen.
func buch(t *testing.T) *Buch {
	t.Helper()
	t.Setenv("ESSTRACKER_DATEN", t.TempDir())
	b, err := Oeffne()
	if err != nil {
		t.Fatalf("Oeffne: %v", err)
	}
	return b
}

func eintrag(id string, menge, kcal float64) Eintrag {
	return Eintrag{
		ID: id, Name: "Haferflocken", Menge: menge, Einheit: "g",
		Zeit: time.Now().Format(time.RFC3339), Mahlzeit: "fruehstueck",
		Je100: Naehrwerte{Kcal: kcal, Fett: 7, Kohlenhydrate: 58.7, Eiweiss: 13.5},
	}
}

// nah vergleicht mit Spielraum. Exakt geht nicht: 7 * 0,8 ist im
// Binaerformat 5,6000000000000005, und das ist kein Fehler, sondern wie
// Gleitkomma rechnet. Gerundet wird bewusst erst an der Anzeige und in der
// CSV — hier drin bleiben die Werte so genau, wie sie hereinkamen.
func nah(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestMalRechnetAufDieMenge(t *testing.T) {
	n := Naehrwerte{Kcal: 372, Fett: 7}
	g := n.Mal(80)
	if !nah(g.Kcal, 297.6) {
		t.Errorf("Kcal = %v, erwartet 297.6", g.Kcal)
	}
	if !nah(g.Fett, 5.6) {
		t.Errorf("Fett = %v, erwartet 5.6", g.Fett)
	}
}

func TestPlusAddiert(t *testing.T) {
	a := Naehrwerte{Kcal: 100, Fett: 2, Eiweiss: 5}
	b := Naehrwerte{Kcal: 250, Fett: 8, Eiweiss: 1.5}
	s := a.Plus(b)
	if !nah(s.Kcal, 350) || !nah(s.Fett, 10) || !nah(s.Eiweiss, 6.5) {
		t.Errorf("Plus = %+v", s)
	}
}

// Derselbe Eintrag zweimal darf nur einmal liegen — darauf verlaesst sich die
// Warteschlange im Handy, die nachschickt, bis der Rechner bestaetigt.
func TestAufnehmenIstWiederholbar(t *testing.T) {
	b := buch(t)

	neu, err := b.Aufnehmen(eintrag("a1", 80, 372), "handy")
	if err != nil || !neu {
		t.Fatalf("erste Aufnahme: neu=%v err=%v", neu, err)
	}
	neu, err = b.Aufnehmen(eintrag("a1", 80, 372), "handy")
	if err != nil {
		t.Fatalf("zweite Aufnahme: %v", err)
	}
	if neu {
		t.Error("zweite Aufnahme meldet neu=true — der Eintrag laege doppelt")
	}
	if len(b.Eintraege) != 1 {
		t.Errorf("%d Eintraege, erwartet 1", len(b.Eintraege))
	}
}

func TestAufnehmenWeistUnfugAb(t *testing.T) {
	b := buch(t)
	faelle := map[string]Eintrag{
		"ohne ID":       {Name: "Brot", Menge: 50},
		"ohne Namen":    {ID: "x", Menge: 50},
		"Menge null":    {ID: "x", Name: "Brot", Menge: 0},
		"Menge negativ": {ID: "x", Name: "Brot", Menge: -5},
		"Menge absurd":  {ID: "x", Name: "Brot", Menge: 200000},
		"krumme Zeit":   {ID: "x", Name: "Brot", Menge: 50, Zeit: "gestern"},
	}
	for name, e := range faelle {
		if _, err := b.Aufnehmen(e, "handy"); err == nil {
			t.Errorf("%s: wurde angenommen, sollte abgewiesen werden", name)
		}
	}
	if len(b.Eintraege) != 0 {
		t.Errorf("%d Eintraege durchgekommen, erwartet 0", len(b.Eintraege))
	}
}

// Der Tag kommt vom Handy, weil nur das Handy die Zone kennt, in der gegessen
// wurde. Ein Eintrag um 23:30 in Tokio gehoert zum dortigen Tag, auch wenn
// hier noch Nachmittag ist.
func TestTagKommtVomHandy(t *testing.T) {
	b := buch(t)
	e := eintrag("a1", 80, 372)
	e.Zeit = "2026-09-15T23:30:00+09:00"
	e.Tag = "2026-09-15"
	if _, err := b.Aufnehmen(e, "handy"); err != nil {
		t.Fatalf("Aufnehmen: %v", err)
	}
	if b.Eintraege[0].Tag != "2026-09-15" {
		t.Errorf("Tag = %q, erwartet 2026-09-15", b.Eintraege[0].Tag)
	}
}

func TestSichtZaehltDenTagZusammen(t *testing.T) {
	b := buch(t)
	heute := Heute()
	for i, m := range []float64{80, 200, 40} {
		e := eintrag(string(rune('a'+i)), m, 100)
		e.Tag = heute
		e.Zeit = heute + "T12:0" + string(rune('0'+i)) + ":00+02:00"
		if _, err := b.Aufnehmen(e, "handy"); err != nil {
			t.Fatalf("Aufnehmen: %v", err)
		}
	}
	s := b.Sicht()
	// 80 + 200 + 40 = 320 g bei 100 kcal je 100 g
	if s.Zahlen.Kcal != 320 {
		t.Errorf("Kcal = %d, erwartet 320", s.Zahlen.Kcal)
	}
	if s.Zahlen.Eintraege != 3 {
		t.Errorf("Eintraege = %d, erwartet 3", s.Zahlen.Eintraege)
	}
	if s.Zahlen.Rest != s.Ziel-320 {
		t.Errorf("Rest = %d, erwartet %d", s.Zahlen.Rest, s.Ziel-320)
	}
	if len(s.Woche) != 7 {
		t.Errorf("Woche hat %d Punkte, erwartet 7", len(s.Woche))
	}
	// Das Raster endet am kommenden Sonntag und deckt 53 volle Wochen ab.
	if len(s.Raster)%7 != 0 {
		t.Errorf("Raster hat %d Felder — keine ganzen Wochen", len(s.Raster))
	}
}

// Nachgeschickte Eintraege muessen sich in die Reihenfolge des Essens
// einsortieren, nicht hinten anhaengen.
func TestNachzueglerLandetAnDerRichtigenStelle(t *testing.T) {
	b := buch(t)
	heute := Heute()

	spaet := eintrag("spaet", 100, 100)
	spaet.Tag, spaet.Zeit = heute, heute+"T19:00:00+02:00"
	frueh := eintrag("frueh", 100, 100)
	frueh.Tag, frueh.Zeit = heute, heute+"T07:00:00+02:00"

	if _, err := b.Aufnehmen(spaet, "handy"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Aufnehmen(frueh, "handy"); err != nil {
		t.Fatal(err)
	}
	if b.Eintraege[0].ID != "frueh" {
		t.Errorf("erster Eintrag ist %q, erwartet frueh", b.Eintraege[0].ID)
	}
}

func TestStufeHaengtAmZiel(t *testing.T) {
	faelle := []struct {
		kcal, ziel, will int
	}{
		{0, 2000, 0},
		{500, 2000, 1},  // 25 %
		{1000, 2000, 2}, // 50 %
		{1800, 2000, 3}, // 90 %
		{2000, 2000, 4}, // voll
		{2600, 2000, 4}, // darueber bleibt voll
		{1000, 4000, 1}, // dieselbe Menge, hoeheres Ziel, niedrigere Stufe
	}
	for _, f := range faelle {
		if s := stufe(f.kcal, f.ziel); s != f.will {
			t.Errorf("stufe(%d, %d) = %d, erwartet %d", f.kcal, f.ziel, s, f.will)
		}
	}
}

func TestZielBleibtInGrenzen(t *testing.T) {
	b := buch(t)
	if err := b.ZielSetzen(2400); err != nil {
		t.Fatalf("2400 abgelehnt: %v", err)
	}
	if b.Tagesziel() != 2400 {
		t.Errorf("Tagesziel = %d, erwartet 2400", b.Tagesziel())
	}
	for _, unfug := range []int{0, -100, 499, 20001} {
		if err := b.ZielSetzen(unfug); err == nil {
			t.Errorf("%d wurde angenommen", unfug)
		}
	}
	if b.Tagesziel() != 2400 {
		t.Errorf("Tagesziel nach abgelehnten Versuchen = %d, erwartet 2400", b.Tagesziel())
	}
}

// Ein getrenntes Geraet nimmt seine Eintraege nicht mit: die beschreiben
// gegessenes Essen, nicht ein Geraet.
func TestGeraetLoesenBehaeltEintraege(t *testing.T) {
	b := buch(t)
	if err := b.GeraetMerken(Geraet{ID: "h1", Name: "Handy", Token: "geheim"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Aufnehmen(eintrag("a1", 80, 372), "h1"); err != nil {
		t.Fatal(err)
	}
	if err := b.GeraetLoesen("h1"); err != nil {
		t.Fatal(err)
	}
	if len(b.Eintraege) != 1 {
		t.Errorf("%d Eintraege, erwartet 1", len(b.Eintraege))
	}
	if _, da := b.GeraetZuToken("geheim"); da {
		t.Error("Token gilt nach dem Trennen noch")
	}
}

func TestLeeresTokenTrifftNie(t *testing.T) {
	b := buch(t)
	if err := b.GeraetMerken(Geraet{ID: "h1", Name: "Handy", Token: ""}); err != nil {
		t.Fatal(err)
	}
	if _, da := b.GeraetZuToken(""); da {
		t.Error("leeres Token wurde einem Geraet zugeordnet")
	}
}
