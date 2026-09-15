package tagebuch

import (
	"sort"
	"strings"
	"time"
)

// Die Oberflaeche rechnet nichts aus. Jede Anfrage an den Dienst — auch jede
// aendernde — bekommt den vollstaendigen Stand zurueck und zeichnet ihn neu.
// Alles, was hier drinsteht, ist daher abgeleitet und wird nie gespeichert:
// es gibt keinen zweiten Ort, an dem eine Tagessumme entsteht, und damit auch
// keinen, an dem sie auseinanderlaufen kann.

// Sicht ist der Stand, wie ihn die Oberflaeche braucht.
type Sicht struct {
	Heute    string    `json:"heute"`
	Ziel     int       `json:"ziel"`
	Zahlen   Zahlen    `json:"zahlen"`
	Tage     []Tag     `json:"tage"`
	Woche    []Punkt   `json:"woche"`
	Produkte []Produkt `json:"produkte"`
	Raster   []Feld    `json:"raster"`
	Geraete  []Geraet  `json:"geraete"`
	Kopplung Kopplung  `json:"kopplung"`
}

// Zahlen ist der heutige Stand — die einzige Rechnung, die auch das Handy
// anstellt, und deshalb die einzige, die zweimal existiert.
type Zahlen struct {
	Kcal       int        `json:"kcal"`
	Ziel       int        `json:"ziel"`
	Prozent    int        `json:"prozent"`
	Rest       int        `json:"rest"`
	Eintraege  int        `json:"eintraege"`
	Naehrwerte Naehrwerte `json:"naehrwerte"`
	Schnitt7   int        `json:"schnitt7"` // Mittel der letzten sieben Tage mit Eintraegen
	Tage       int        `json:"tage"`     // Tage mit mindestens einem Eintrag
}

// Tag fasst einen Kalendertag zusammen.
type Tag struct {
	Tag        string     `json:"tag"`
	Kcal       int        `json:"kcal"`
	Prozent    int        `json:"prozent"`
	Naehrwerte Naehrwerte `json:"naehrwerte"`
	Zeilen     []Zeile    `json:"zeilen"`
}

// Zeile ist ein Eintrag, fertig zum Zeichnen.
type Zeile struct {
	Eintrag
	Kcal   int        `json:"kcal"`
	Gesamt Naehrwerte `json:"gesamt"`
	Uhr    string     `json:"uhr"` // HH:MM in der Zone, in der gegessen wurde
}

// Punkt ist ein Tag im Balken der letzten Woche.
type Punkt struct {
	Tag     string `json:"tag"`
	Kcal    int    `json:"kcal"`
	Prozent int    `json:"prozent"`
	Heute   bool   `json:"heute"`
}

// Produkt ist ein Name, so oft wie er vorkommt.
type Produkt struct {
	Name    string  `json:"name"`
	Marke   string  `json:"marke,omitempty"`
	Barcode string  `json:"barcode,omitempty"`
	Anzahl  int     `json:"anzahl"`
	Kcal100 float64 `json:"kcal100"`
	Menge   float64 `json:"menge"` // insgesamt gegessen
	Einheit string  `json:"einheit"`
	Zuletzt string  `json:"zuletzt"`
}

// Feld ist ein Tag im Jahresraster. Die Stufe laeuft ueber die Naehe zum
// Ziel, nicht ueber die reine Menge: ein Tag mit 2000 von 2000 kcal ist voll,
// einer mit 2000 von 3500 nicht.
type Feld struct {
	Tag   string `json:"tag"`
	Kcal  int    `json:"kcal"`
	Stufe int    `json:"stufe"` // 0 nichts, 1..4 steigend
}

// Kopplung sagt, ob gerade ein Handy dazukommen darf.
type Kopplung struct {
	Offen    bool     `json:"offen"`
	BisSek   int      `json:"bisSek"`
	Wort     string   `json:"wort,omitempty"`
	Adressen []string `json:"adressen"`
	Port     int      `json:"port"`
}

// Sicht baut den Stand.
func (b *Buch) Sicht() Sicht {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	heute := Heute()
	s := Sicht{Heute: heute, Ziel: b.Ziel, Geraete: b.Geraete}
	if s.Geraete == nil {
		s.Geraete = []Geraet{}
	}

	// Nach Tagen buendeln. Die Eintraege liegen bereits nach Zeit sortiert,
	// die Reihenfolge innerhalb eines Tages stimmt damit schon.
	stelle := map[string]int{}
	for _, e := range b.Eintraege {
		i, da := stelle[e.Tag]
		if !da {
			s.Tage = append(s.Tage, Tag{Tag: e.Tag})
			i = len(s.Tage) - 1
			stelle[e.Tag] = i
		}
		g := e.Gesamt()
		s.Tage[i].Zeilen = append(s.Tage[i].Zeilen, Zeile{
			Eintrag: e, Kcal: runde(g.Kcal), Gesamt: g, Uhr: uhr(e.Zeit),
		})
		s.Tage[i].Naehrwerte = s.Tage[i].Naehrwerte.Plus(g)
	}
	for i := range s.Tage {
		s.Tage[i].Kcal = runde(s.Tage[i].Naehrwerte.Kcal)
		s.Tage[i].Prozent = anteil(s.Tage[i].Kcal, b.Ziel)
	}
	// Neueste zuerst: die Oberflaeche liest von oben, und der heutige Tag
	// ist der, wegen dem das Fenster offen ist.
	sort.Slice(s.Tage, func(i, j int) bool { return s.Tage[i].Tag > s.Tage[j].Tag })
	if s.Tage == nil {
		s.Tage = []Tag{}
	}

	kcalAmTag := map[string]int{}
	for _, t := range s.Tage {
		kcalAmTag[t.Tag] = t.Kcal
	}

	// Heute. Ueber den Tag suchen, nicht ueber stelle: dessen Indizes zeigen
	// auf die Liste vor dem Sortieren und stimmen jetzt nicht mehr.
	for _, t := range s.Tage {
		if t.Tag == heute {
			s.Zahlen = Zahlen{
				Kcal: t.Kcal, Eintraege: len(t.Zeilen), Naehrwerte: t.Naehrwerte,
			}
			break
		}
	}
	s.Zahlen.Ziel = b.Ziel
	s.Zahlen.Prozent = anteil(s.Zahlen.Kcal, b.Ziel)
	s.Zahlen.Rest = b.Ziel - s.Zahlen.Kcal
	s.Zahlen.Tage = len(s.Tage)

	// Schnitt der letzten sieben Tage mit Eintraegen. Leere Tage zaehlen
	// nicht mit: wer zwei Tage nicht eingetragen hat, hat nicht zwei Tage
	// nichts gegessen, und ein Mittelwert, der das behauptet, ist wertlos.
	summe, zahl := 0, 0
	for _, t := range s.Tage {
		if zahl == 7 {
			break
		}
		summe += t.Kcal
		zahl++
	}
	if zahl > 0 {
		s.Zahlen.Schnitt7 = summe / zahl
	}

	// Die letzten sieben Kalendertage, leere eingeschlossen — hier ist die
	// Luecke die Aussage.
	jetzt := time.Now()
	for i := 6; i >= 0; i-- {
		tag := jetzt.AddDate(0, 0, -i).Format("2006-01-02")
		kcal := kcalAmTag[tag]
		s.Woche = append(s.Woche, Punkt{
			Tag: tag, Kcal: kcal, Prozent: anteil(kcal, b.Ziel), Heute: tag == heute,
		})
	}

	s.Produkte = produkte(b.Eintraege)
	s.Raster = raster(kcalAmTag, b.Ziel, jetzt)
	return s
}

// produkte zaehlt, was wie oft gegessen wurde — gebuendelt ueber den Barcode,
// wo es einen gibt, sonst ueber Name und Marke.
func produkte(eintraege []Eintrag) []Produkt {
	stelle := map[string]int{}
	var liste []Produkt
	for _, e := range eintraege {
		schluessel := e.Barcode
		if schluessel == "" {
			schluessel = strings.ToLower(e.Name + "|" + e.Marke)
		}
		i, da := stelle[schluessel]
		if !da {
			liste = append(liste, Produkt{
				Name: e.Name, Marke: e.Marke, Barcode: e.Barcode,
				Kcal100: e.Je100.Kcal, Einheit: e.Einheit,
			})
			i = len(liste) - 1
			stelle[schluessel] = i
		}
		liste[i].Anzahl++
		liste[i].Menge += e.Menge
		if e.Tag > liste[i].Zuletzt {
			liste[i].Zuletzt = e.Tag
		}
	}
	sort.Slice(liste, func(i, j int) bool {
		if liste[i].Anzahl != liste[j].Anzahl {
			return liste[i].Anzahl > liste[j].Anzahl
		}
		return liste[i].Zuletzt > liste[j].Zuletzt
	})
	if liste == nil {
		liste = []Produkt{}
	}
	return liste
}

// raster baut ein Jahr in Wochenspalten, Montag oben — dieselbe Form wie das
// Raster in der Tasklist nebenan.
func raster(kcalAmTag map[string]int, ziel int, jetzt time.Time) []Feld {
	ende := jetzt
	// Bis zum kommenden Sonntag auffuellen, damit die letzte Spalte
	// vollstaendig ist und das Raster nicht ausfranst.
	for ende.Weekday() != time.Sunday {
		ende = ende.AddDate(0, 0, 1)
	}
	start := ende.AddDate(0, 0, -7*52-6)

	var felder []Feld
	for t := start; !t.After(ende); t = t.AddDate(0, 0, 1) {
		tag := t.Format("2006-01-02")
		kcal := kcalAmTag[tag]
		felder = append(felder, Feld{Tag: tag, Kcal: kcal, Stufe: stufe(kcal, ziel)})
	}
	return felder
}

// stufe teilt einen Tag in fuenf Helligkeiten. Die Schwellen haengen am Ziel,
// damit das Raster bei jedem Tagesziel dieselbe Aussage hat.
func stufe(kcal, ziel int) int {
	if kcal <= 0 || ziel <= 0 {
		return 0
	}
	p := kcal * 100 / ziel
	switch {
	case p < 35:
		return 1
	case p < 70:
		return 2
	case p < 100:
		return 3
	default:
		return 4
	}
}

func anteil(ist, soll int) int {
	if soll <= 0 {
		return 0
	}
	return ist * 100 / soll
}

func runde(f float64) int {
	if f < 0 {
		return -int(-f + 0.5)
	}
	return int(f + 0.5)
}

// uhr zieht die Uhrzeit aus der Zeit, in der Zone, in der gegessen wurde.
func uhr(zeit string) string {
	t, err := time.Parse(time.RFC3339, zeit)
	if err != nil {
		return ""
	}
	return t.Format("15:04")
}
