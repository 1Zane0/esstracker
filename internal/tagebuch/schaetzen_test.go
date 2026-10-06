package tagebuch

import "testing"

// offen baut, was das Handy schickt, wenn nur Name und Menge bekannt sind.
func offen(id, name string, menge float64) Eintrag {
	e := eintrag(id, menge, 0)
	e.Name = name
	e.Je100 = Naehrwerte{}
	e.Quelle = QuelleOffen
	return e
}

// Die Quelle bestimmt der Rechner. Ein selbst eingetippter Eintrag ohne
// Kilokalorien ist offen, auch wenn das Handy "hand" sagt — und kein Handy
// kann behaupten, Claude haette schon geschaetzt.
func TestQuelleBestimmtDerRechner(t *testing.T) {
	faelle := []struct {
		name   string
		quelle string
		kcal   float64
		will   string
	}{
		{"offen ohne Werte", QuelleOffen, 0, QuelleOffen},
		{"hand ohne Werte", QuelleHand, 0, QuelleOffen},
		{"hand mit Werten", QuelleHand, 57, QuelleHand},
		{"offen mit Werten", QuelleOffen, 57, QuelleHand},
		{"Datenbank", QuelleOFF, 372, QuelleOFF},
		{"leer", "", 372, QuelleOFF},
		{"vorgeblich geschaetzt", QuelleKI, 57, QuelleOFF},
	}
	for _, f := range faelle {
		b := buch(t)
		e := eintrag("a1", 100, f.kcal)
		e.Quelle = f.quelle
		e.Schaetzung = &Schaetzung{Begruendung: "vom Handy untergeschoben"}
		if _, err := b.Aufnehmen(e, "handy"); err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		if q := b.Eintraege[0].Quelle; q != f.will {
			t.Errorf("%s: Quelle = %q, erwartet %q", f.name, q, f.will)
		}
		if b.Eintraege[0].Schaetzung != nil {
			t.Errorf("%s: Schaetzung vom Handy wurde uebernommen", f.name)
		}
	}
}

func TestOffenerEintragZaehltNullUndStehtInDerSicht(t *testing.T) {
	b := buch(t)
	e := offen("o1", "kulturheidelbeeren", 150)
	e.Tag = Heute()
	if _, err := b.Aufnehmen(e, "handy"); err != nil {
		t.Fatal(err)
	}
	s := b.Sicht()
	if s.Zahlen.Kcal != 0 {
		t.Errorf("Kcal = %d, erwartet 0", s.Zahlen.Kcal)
	}
	if len(s.Offen) != 1 || s.Offen[0].ID != "o1" {
		t.Errorf("Offen = %+v, erwartet genau o1", s.Offen)
	}
}

// Was Claude schaetzt, zaehlt danach im Tag — und was am Handy schon
// angegeben war, bleibt stehen.
func TestSchaetzenTraegtEinUndBehaeltAngegebenes(t *testing.T) {
	b := buch(t)
	e := offen("o1", "kulturheidelbeeren", 200)
	e.Tag = Heute()
	e.Je100.Eiweiss = 1.5 // am Handy eingetippt
	if _, err := b.Aufnehmen(e, "handy"); err != nil {
		t.Fatal(err)
	}

	err := b.Schaetzen("o1", Naehrwerte{Kcal: 57, Kohlenhydrate: 14.5, Zucker: 10, Eiweiss: 0.7},
		"Kulturheidelbeeren", Schaetzung{Sicherheit: "hoch", Begruendung: "Tabellenwert"})
	if err != nil {
		t.Fatalf("Schaetzen: %v", err)
	}

	got := b.Eintraege[0]
	if got.Quelle != QuelleKI {
		t.Errorf("Quelle = %q, erwartet ki", got.Quelle)
	}
	if !nah(got.Je100.Kcal, 57) || !nah(got.Je100.KJ, 57*4.184) {
		t.Errorf("Kcal/KJ = %v/%v", got.Je100.Kcal, got.Je100.KJ)
	}
	if !nah(got.Je100.Eiweiss, 1.5) {
		t.Errorf("Eiweiss = %v — die Angabe vom Handy wurde ueberschrieben", got.Je100.Eiweiss)
	}
	if got.Name != "Kulturheidelbeeren" || got.Schaetzung == nil || got.Schaetzung.NameVorher != "kulturheidelbeeren" {
		t.Errorf("Name = %q, Schaetzung = %+v", got.Name, got.Schaetzung)
	}
	if s := b.Sicht(); s.Zahlen.Kcal != 114 || len(s.Offen) != 0 {
		t.Errorf("nach dem Schaetzen: Kcal = %d (erwartet 114), offen = %d", s.Zahlen.Kcal, len(s.Offen))
	}

	// Ein zweites Mal geht nicht: der Eintrag ist nicht mehr offen.
	if err := b.Schaetzen("o1", Naehrwerte{Kcal: 999}, "", Schaetzung{}); err == nil {
		t.Error("ein geschaetzter Eintrag liess sich noch einmal schaetzen")
	}
}

// Das Handy schickt nach, bis der Rechner bestaetigt. Kommt derselbe Eintrag
// nach der Schaetzung noch einmal, darf er sie nicht zurueck auf null setzen.
func TestNachzueglerUeberschreibtSchaetzungNicht(t *testing.T) {
	b := buch(t)
	if _, err := b.Aufnehmen(offen("o1", "Döner", 350), "handy"); err != nil {
		t.Fatal(err)
	}
	if err := b.Schaetzen("o1", Naehrwerte{Kcal: 215}, "", Schaetzung{}); err != nil {
		t.Fatal(err)
	}
	neu, err := b.Aufnehmen(offen("o1", "Döner", 350), "handy")
	if err != nil || neu {
		t.Fatalf("Nachzuegler: neu=%v err=%v", neu, err)
	}
	if b.Eintraege[0].Quelle != QuelleKI || !nah(b.Eintraege[0].Je100.Kcal, 215) {
		t.Errorf("nach dem Nachzuegler: %+v", b.Eintraege[0])
	}
}

// Freigeben ist fuer die Verlegenheitszahl: am Handy 1 kcal, weil das Feld
// nicht leer bleiben durfte. Die Eingabe bleibt nachlesbar.
func TestFreigebenMachtWiederOffen(t *testing.T) {
	b := buch(t)
	e := eintrag("h1", 100, 1)
	e.Name = "kulturheidelbeeren"
	e.Quelle = QuelleHand
	if _, err := b.Aufnehmen(e, "handy"); err != nil {
		t.Fatal(err)
	}

	if err := b.Freigeben("h1"); err != nil {
		t.Fatalf("Freigeben: %v", err)
	}
	got := b.Eintraege[0]
	if got.Quelle != QuelleOffen || got.Je100.Kcal != 0 {
		t.Errorf("nach Freigeben: Quelle %q, Kcal %v", got.Quelle, got.Je100.Kcal)
	}
	if got.Vorher == nil || got.Vorher.Je100.Kcal != 1 || got.Vorher.Quelle != QuelleHand {
		t.Errorf("Vorher = %+v, erwartet die 1 kcal von Hand", got.Vorher)
	}

	// Geschaetzt, dann noch einmal verworfen: Vorher bleibt die Eingabe vom
	// Handy, und der Name kehrt zu dem zurueck, was dort stand.
	if err := b.Schaetzen("h1", Naehrwerte{Kcal: 57}, "Kulturheidelbeeren", Schaetzung{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Freigeben("h1"); err != nil {
		t.Fatal(err)
	}
	got = b.Eintraege[0]
	if got.Vorher.Je100.Kcal != 1 || got.Name != "kulturheidelbeeren" || got.Schaetzung != nil {
		t.Errorf("nach zweitem Freigeben: Name %q, Vorher %+v, Schaetzung %+v", got.Name, got.Vorher, got.Schaetzung)
	}
}

func TestDatenbankwerteWerdenNichtFreigegeben(t *testing.T) {
	b := buch(t)
	if _, err := b.Aufnehmen(eintrag("d1", 80, 372), "handy"); err != nil {
		t.Fatal(err)
	}
	if err := b.Freigeben("d1"); err == nil {
		t.Error("ein Eintrag aus Open Food Facts wurde zum Schaetzen freigegeben")
	}
}

// Als Vergleich taugt nur, was feststeht. Eine von Hand getippte 1 kcal soll
// keine neue Schaetzung anstecken.
func TestBekannteLassenHandeingabenWeg(t *testing.T) {
	b := buch(t)
	hand := eintrag("h1", 100, 1)
	hand.Name, hand.Quelle = "Heidelbeeren", QuelleHand
	db := eintrag("d1", 80, 372)
	for _, e := range []Eintrag{hand, db, offen("o1", "Döner", 300)} {
		if _, err := b.Aufnehmen(e, "handy"); err != nil {
			t.Fatal(err)
		}
	}
	bekannt := b.Bekannte(10)
	if len(bekannt) != 1 || bekannt[0].ID != "d1" {
		t.Errorf("Bekannte = %+v, erwartet nur d1", bekannt)
	}
}

// Jede Schaetzung landet in der eigenen Datenbank — dasselbe Lebensmittel
// nur einmal, mit der juengsten Zahl.
func TestSchaetzungLandetInDerEigenenDatenbank(t *testing.T) {
	b := buch(t)
	for _, e := range []Eintrag{offen("o1", "kulturheidelbeeren", 150), offen("o2", "Kulturheidelbeeren", 80)} {
		if _, err := b.Aufnehmen(e, "handy"); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Schaetzen("o1", Naehrwerte{Kcal: 55}, "Kulturheidelbeeren", Schaetzung{Zeit: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Schaetzen("o2", Naehrwerte{Kcal: 57}, "", Schaetzung{Zeit: "t2"}); err != nil {
		t.Fatal(err)
	}
	if len(b.Eigene) != 1 {
		t.Fatalf("%d eigene, erwartet 1: %+v", len(b.Eigene), b.Eigene)
	}
	if e := b.Eigene[0]; e.Je100.Kcal != 57 || e.ID != "e-o1" || e.Stand != "t2" {
		t.Errorf("eigenes = %+v, erwartet 57 kcal unter der ersten ID", e)
	}

	// Kommt derselbe Name spaeter mit Strichcode, wird daraus kein zweites
	// Lebensmittel — der Code kommt zum ersten dazu.
	mitCode := offen("o3", "Kulturheidelbeeren", 125)
	mitCode.Barcode = "4012345678901"
	if _, err := b.Aufnehmen(mitCode, "handy"); err != nil {
		t.Fatal(err)
	}
	if err := b.Schaetzen("o3", Naehrwerte{Kcal: 60}, "", Schaetzung{}); err != nil {
		t.Fatal(err)
	}
	if len(b.Eigene) != 1 || b.Eigene[0].Barcode != "4012345678901" {
		t.Errorf("ohne Code und mit Code gleichen Namens sollten eins werden: %+v", b.Eigene)
	}

	if err := b.EigenesLoeschen("e-o1"); err != nil {
		t.Fatal(err)
	}
	if len(b.Eigene) != 0 || b.Eintraege[0].Je100.Kcal != 55 {
		t.Errorf("nach dem Loeschen: eigene %d, Eintrag %v kcal", len(b.Eigene), b.Eintraege[0].Je100.Kcal)
	}
}

// Das Handy bekommt zurueck, was hier nachgetragen wurde — und nichts von
// dem, was es selbst schon richtig hat.
func TestAbgleichBringtNachtraegeUndEigene(t *testing.T) {
	b := buch(t)
	heute := Heute()
	alt := offen("alt", "Döner", 350)
	alt.Tag = "2020-01-01"
	for _, e := range []Eintrag{offen("o1", "Döner", 350), offen("o2", "Zeug", 50), eintrag("d1", 80, 372), alt} {
		if e.Tag == "" {
			e.Tag = heute
		}
		if _, err := b.Aufnehmen(e, "handy"); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Schaetzen("o1", Naehrwerte{Kcal: 215}, "Döner Kebab", Schaetzung{}); err != nil {
		t.Fatal(err)
	}

	a := b.Abgleichen()
	quelle := map[string]string{}
	for _, n := range a.Nachtraege {
		quelle[n.ID] = n.Quelle
	}
	if quelle["o1"] != QuelleKI || quelle["o2"] != QuelleOffen || len(quelle) != 2 {
		t.Errorf("Nachtraege = %+v, erwartet o1 geschaetzt und o2 offen, nichts Altes, nichts aus der Datenbank", a.Nachtraege)
	}
	if len(a.Eigene) != 1 || a.Eigene[0].Name != "Döner Kebab" {
		t.Errorf("Eigene = %+v", a.Eigene)
	}
}

// Was das Handy aus der eigenen Datenbank waehlt, zaehlt als "eigen" —
// ohne Werte aber waere es offen wie alles andere.
func TestEigenVomHandy(t *testing.T) {
	b := buch(t)
	e := eintrag("x1", 100, 57)
	e.Quelle = QuelleEigen
	if _, err := b.Aufnehmen(e, "handy"); err != nil {
		t.Fatal(err)
	}
	if b.Eintraege[0].Quelle != QuelleEigen {
		t.Errorf("Quelle = %q, erwartet eigen", b.Eintraege[0].Quelle)
	}
}

// Die neue App an einem alten Rechner: dort wurde aus "offen" ein "off" mit
// null Kilokalorien. Beim naechsten Oeffnen muss das wieder offen sein, sonst
// schaetzt es nie jemand.
func TestAlterRechnerMachteOffenZuOff(t *testing.T) {
	b := buch(t)
	b.Eintraege = []Eintrag{
		{ID: "a", Name: "Döner", Menge: 350, Quelle: QuelleOFF},
		{ID: "b", Name: "Haferflocken", Menge: 80, Quelle: QuelleOFF, Je100: Naehrwerte{Kcal: 372}},
	}
	if err := b.sichern(); err != nil {
		t.Fatal(err)
	}
	neu, err := Oeffne()
	if err != nil {
		t.Fatal(err)
	}
	if neu.Eintraege[0].Quelle != QuelleOffen || neu.Eintraege[1].Quelle != QuelleOFF {
		t.Errorf("Quellen nach dem Oeffnen: %q, %q", neu.Eintraege[0].Quelle, neu.Eintraege[1].Quelle)
	}
}
