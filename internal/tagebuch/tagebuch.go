// Package tagebuch haelt, was gegessen wurde.
//
// Ein Eintrag ist unveraenderlich, sobald er hier ankommt: er beschreibt ein
// Ereignis in der Vergangenheit. Geaendert wird nur, was nicht zum Ereignis
// gehoert — das Tagesziel und die gekoppelten Geraete.
//
// Zwei Entscheidungen, die den Rest einfach machen:
//
// Erstens traegt jeder Eintrag eine ID, die das Handy vergibt. Ein Eintrag,
// der schon liegt, wird beim zweiten Mal nicht noch einmal angelegt. Damit
// darf das Handy jeden Eintrag beliebig oft nachschicken, ohne mitzaehlen zu
// muessen, was angekommen ist — und genau das braucht eine Warteschlange, die
// einen ausgeschalteten Rechner ueberleben soll.
//
// Zweitens stehen hier nur die Naehrwerte je 100 g, so wie sie in der
// Datenbank stehen. Was eine Portion ergibt, wird gerechnet, nicht gespeichert.
// Sonst gaebe es zwei Zahlen, die dasselbe behaupten, und irgendwann behaupten
// sie Verschiedenes.
package tagebuch

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/1Zane0/esstracker/internal/speicher"
)

const datei = "tagebuch.json"

// ZielVorgabe ist das Tagesziel, solange keines gesetzt wurde. 2000 kcal ist
// der Wert, auf den sich Naehrwerttabellen beziehen — als Platzhalter ehrlicher
// als eine Zahl, die persoenlich aussieht, ohne es zu sein.
const ZielVorgabe = 2000

// Naehrwerte sind die Angaben je 100 g beziehungsweise 100 ml.
//
// Alles ausser den beiden Energiewerten ist in Gramm. Fehlende Angaben sind
// null — Open Food Facts ist eine Sammlung von Freiwilligen, und bei vielen
// Produkten steht nur die Energie drin.
type Naehrwerte struct {
	Kcal          float64 `json:"kcal"`
	KJ            float64 `json:"kj"`
	Fett          float64 `json:"fett"`
	Gesaettigt    float64 `json:"gesaettigt"`
	Kohlenhydrate float64 `json:"kohlenhydrate"`
	Zucker        float64 `json:"zucker"`
	Ballaststoffe float64 `json:"ballaststoffe"`
	Eiweiss       float64 `json:"eiweiss"`
	Salz          float64 `json:"salz"`
}

// Mal rechnet die Werte auf eine Menge hoch. Die Angaben beziehen sich auf
// 100 g, die Menge steht in Gramm.
func (n Naehrwerte) Mal(menge float64) Naehrwerte {
	f := menge / 100
	return Naehrwerte{
		Kcal: n.Kcal * f, KJ: n.KJ * f,
		Fett: n.Fett * f, Gesaettigt: n.Gesaettigt * f,
		Kohlenhydrate: n.Kohlenhydrate * f, Zucker: n.Zucker * f,
		Ballaststoffe: n.Ballaststoffe * f, Eiweiss: n.Eiweiss * f,
		Salz: n.Salz * f,
	}
}

// Plus addiert zwei Saetze Naehrwerte.
func (n Naehrwerte) Plus(m Naehrwerte) Naehrwerte {
	return Naehrwerte{
		Kcal: n.Kcal + m.Kcal, KJ: n.KJ + m.KJ,
		Fett: n.Fett + m.Fett, Gesaettigt: n.Gesaettigt + m.Gesaettigt,
		Kohlenhydrate: n.Kohlenhydrate + m.Kohlenhydrate, Zucker: n.Zucker + m.Zucker,
		Ballaststoffe: n.Ballaststoffe + m.Ballaststoffe, Eiweiss: n.Eiweiss + m.Eiweiss,
		Salz: n.Salz + m.Salz,
	}
}

// Eintrag ist ein Produkt zu einer Zeit.
type Eintrag struct {
	ID       string `json:"id"`       // vom Handy vergeben, entscheidet ueber Doppelte
	Zeit     string `json:"zeit"`     // RFC3339 mit Zone: wann gegessen
	Tag      string `json:"tag"`      // JJJJ-MM-TT, vom Handy in seiner Zone gebildet
	Mahlzeit string `json:"mahlzeit"` // fruehstueck | mittag | abend | snack

	Name    string  `json:"name"`
	Marke   string  `json:"marke,omitempty"`
	Barcode string  `json:"barcode,omitempty"`
	Menge   float64 `json:"menge"`   // in Gramm oder Milliliter
	Einheit string  `json:"einheit"` // g | ml

	Je100  Naehrwerte `json:"je100"`
	Quelle string     `json:"quelle"` // off | hand

	// Herkunft, damit spaeter nachvollziehbar ist, wo eine Zahl herkommt.
	Geraet    string `json:"geraet,omitempty"`
	Empfangen string `json:"empfangen"` // wann dieser Rechner ihn bekam
}

// Gesamt sind die Naehrwerte der gegessenen Menge.
func (e Eintrag) Gesamt() Naehrwerte { return e.Je100.Mal(e.Menge) }

// Geraet ist ein gekoppeltes Handy.
type Geraet struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	Gekoppelt string `json:"gekoppelt"`
	Zuletzt   string `json:"zuletzt,omitempty"`
	Eintraege int    `json:"eintraege"`
}

// Buch ist der gesamte Stand.
type Buch struct {
	sperre sync.Mutex

	Eintraege []Eintrag `json:"eintraege"`
	Ziel      int       `json:"ziel"`
	Geraete   []Geraet  `json:"geraete"`
}

// Oeffne laedt das Tagebuch von der Platte.
func Oeffne() (*Buch, error) {
	b := &Buch{Ziel: ZielVorgabe}
	if err := speicher.Lies(datei, b); err != nil {
		return nil, err
	}
	if b.Ziel <= 0 {
		b.Ziel = ZielVorgabe
	}
	return b, nil
}

func (b *Buch) sichern() error { return speicher.Schreib(datei, b) }

// Heute ist der heutige Tag in der Zone dieses Rechners.
func Heute() string { return time.Now().Format("2006-01-02") }

// ---------- Eintragen ----------

// Aufnehmen legt einen Eintrag ab und meldet, ob er neu war.
//
// Kennt das Buch die ID schon, passiert nichts und neu ist false. Das ist
// kein Fehler: das Handy schickt nach, bis der Rechner bestaetigt, und darf
// dabei nicht wissen muessen, was schon liegt.
func (b *Buch) Aufnehmen(e Eintrag, geraet string) (neu bool, err error) {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	if err := pruefe(&e); err != nil {
		return false, err
	}
	for _, alt := range b.Eintraege {
		if alt.ID == e.ID {
			return false, nil
		}
	}
	e.Geraet = geraet
	e.Empfangen = time.Now().Format(time.RFC3339)
	b.Eintraege = append(b.Eintraege, e)

	// Nach Zeit sortiert halten: die Oberflaeche zeigt Tage und will
	// innerhalb eines Tages die Reihenfolge des Essens, nicht die des
	// Empfangs. Nachgeschickte Eintraege kommen sonst hinten an.
	sort.SliceStable(b.Eintraege, func(i, j int) bool { return b.Eintraege[i].Zeit < b.Eintraege[j].Zeit })

	for i := range b.Geraete {
		if b.Geraete[i].ID == geraet {
			b.Geraete[i].Zuletzt = e.Empfangen
			b.Geraete[i].Eintraege++
		}
	}
	return true, b.sichern()
}

// pruefe haelt Unfug draussen und fuellt auf, was sich ableiten laesst.
func pruefe(e *Eintrag) error {
	e.ID = strings.TrimSpace(e.ID)
	e.Name = strings.TrimSpace(e.Name)
	if e.ID == "" {
		return errors.New("Eintrag ohne ID")
	}
	if e.Name == "" {
		return errors.New("Eintrag ohne Namen")
	}
	if e.Menge <= 0 {
		return errors.New("Menge muss groesser als null sein")
	}
	if e.Menge > 100000 {
		return errors.New("Menge ist unglaubwuerdig gross")
	}
	if e.Einheit != "ml" {
		e.Einheit = "g"
	}
	if e.Zeit == "" {
		e.Zeit = time.Now().Format(time.RFC3339)
	}
	wann, err := time.Parse(time.RFC3339, e.Zeit)
	if err != nil {
		return fmt.Errorf("Zeit %q ist kein RFC3339", e.Zeit)
	}
	// Der Tag kommt vom Handy, weil nur das Handy weiss, in welcher Zone
	// gegessen wurde. Fehlt er, ist die Zone aus der Zeit die beste Quelle —
	// die des Rechners waere die schlechteste.
	if e.Tag == "" {
		e.Tag = wann.Format("2006-01-02")
	}
	if !mahlzeiten[e.Mahlzeit] {
		e.Mahlzeit = "snack"
	}
	if e.Quelle != "hand" {
		e.Quelle = "off"
	}
	return nil
}

var mahlzeiten = map[string]bool{"fruehstueck": true, "mittag": true, "abend": true, "snack": true}

// Loeschen nimmt einen Eintrag zurueck — der einzige Weg, einen Fehlgriff auf
// diesem Rechner zu beheben. Das Handy erfaehrt davon nichts; es schickt den
// Eintrag auch nicht erneut, weil seine Warteschlange ihn laengst abgehakt hat.
func (b *Buch) Loeschen(id string) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i, e := range b.Eintraege {
		if e.ID == id {
			b.Eintraege = append(b.Eintraege[:i], b.Eintraege[i+1:]...)
			return b.sichern()
		}
	}
	return errors.New("kein Eintrag mit dieser ID")
}

// ZielSetzen aendert das Tagesziel in Kilokalorien.
func (b *Buch) ZielSetzen(kcal int) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	if kcal < 500 || kcal > 20000 {
		return errors.New("Tagesziel muss zwischen 500 und 20000 kcal liegen")
	}
	b.Ziel = kcal
	return b.sichern()
}

// ---------- Geraete ----------

// GeraetMerken legt ein gekoppeltes Handy ab oder frischt es auf.
func (b *Buch) GeraetMerken(g Geraet) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i := range b.Geraete {
		if b.Geraete[i].ID == g.ID {
			b.Geraete[i].Name = g.Name
			b.Geraete[i].Token = g.Token
			b.Geraete[i].Gekoppelt = g.Gekoppelt
			return b.sichern()
		}
	}
	b.Geraete = append(b.Geraete, g)
	return b.sichern()
}

// GeraetLoesen trennt ein Handy. Seine Eintraege bleiben — sie beschreiben
// gegessenes Essen, nicht ein Geraet.
func (b *Buch) GeraetLoesen(id string) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i, g := range b.Geraete {
		if g.ID == id {
			b.Geraete = append(b.Geraete[:i], b.Geraete[i+1:]...)
			return b.sichern()
		}
	}
	return errors.New("kein Geraet mit dieser ID")
}

// GeraetZuToken sucht das Geraet zu einem Token. Der Vergleich laeuft ueber
// alle Geraete und in konstanter Zeit, damit die Dauer der Antwort nichts
// ueber das richtige Token verraet.
func (b *Buch) GeraetZuToken(token string) (Geraet, bool) {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	var treffer Geraet
	gefunden := false
	for _, g := range b.Geraete {
		if gleich(g.Token, token) {
			treffer, gefunden = g, true
		}
	}
	return treffer, gefunden
}

// gleich vergleicht zwei Zeichenketten in einer Zeit, die nicht davon
// abhaengt, ab welchem Zeichen sie sich unterscheiden.
func gleich(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 && a != ""
}

// Tagesziel ist das Ziel in Kilokalorien. Eigener Zugriff, damit der
// Empfaenger es nennen kann, ohne die ganze Sicht samt Jahresraster zu bauen.
func (b *Buch) Tagesziel() int {
	b.sperre.Lock()
	defer b.sperre.Unlock()
	return b.Ziel
}
