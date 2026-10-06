// Package tagebuch haelt, was gegessen wurde.
//
// Ein Eintrag ist unveraenderlich, sobald er hier ankommt: er beschreibt ein
// Ereignis in der Vergangenheit. Geaendert wird nur, was nicht zum Ereignis
// gehoert — das Tagesziel und die gekoppelten Geraete.
//
// Mit einer Ausnahme: die Naehrwerte eines offenen Eintrags. Wer am Handy nur
// Name und Menge weiss, schickt ihn ohne Werte, und Claude traegt sie hier
// nach. Was gegessen wurde, wann und wie viel, bleibt dabei unberuehrt; die
// Quelle "ki" sagt fuer immer, dass die Zahl geschaetzt ist.
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

// Woher die Naehrwerte eines Eintrags stammen. Die Unterscheidung ist der
// Grund, warum eine geschaetzte Zahl hier neben einer nachgeschlagenen stehen
// darf: man sieht ihr an, was sie ist.
const (
	QuelleOFF   = "off"   // aus Open Food Facts nachgeschlagen
	QuelleHand  = "hand"  // am Handy selbst eingetippt
	QuelleOffen = "offen" // nur Name und Menge — die Naehrwerte fehlen noch
	QuelleKI    = "ki"    // von Claude aus Name und Menge geschaetzt
	QuelleEigen = "eigen" // am Handy aus der eigenen Datenbank gewaehlt
)

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
	Quelle string     `json:"quelle"` // off | hand | offen | ki | eigen

	// Herkunft, damit spaeter nachvollziehbar ist, wo eine Zahl herkommt.
	Geraet    string `json:"geraet,omitempty"`
	Empfangen string `json:"empfangen"` // wann dieser Rechner ihn bekam

	// Schaetzung haelt fest, wie aus einem offenen Eintrag ein gezaehlter
	// wurde — nur bei Quelle "ki".
	Schaetzung *Schaetzung `json:"schaetzung,omitempty"`
	// Vorher sind die Werte, die ein Eintrag trug, bevor er zum Schaetzen
	// freigegeben wurde. Nur gesetzt, wenn es welche gab.
	Vorher *Vorher `json:"vorher,omitempty"`
}

// Schaetzung ist das, was Claude zu einem offenen Eintrag gesagt hat.
type Schaetzung struct {
	Zeit        string `json:"zeit"`        // wann geschaetzt
	Modell      string `json:"modell"`      // welches Modell
	Sicherheit  string `json:"sicherheit"`  // hoch | mittel | niedrig
	Begruendung string `json:"begruendung"` // worauf die Zahl beruht, ein Satz
	NameVorher  string `json:"nameVorher,omitempty"`
}

// Vorher ist der Stand eines Eintrags, bevor er nachgeschaetzt wurde.
type Vorher struct {
	Quelle string     `json:"quelle"`
	Je100  Naehrwerte `json:"je100"`
}

// Gesamt sind die Naehrwerte der gegessenen Menge.
func (e Eintrag) Gesamt() Naehrwerte { return e.Je100.Mal(e.Menge) }

// Eigenes ist ein Lebensmittel in der eigenen Datenbank.
//
// Dort landet, was Claude einmal geschaetzt hat. Das Handy holt sich die Liste
// und findet beim naechsten Mal "Kulturheidelbeeren" in der Suche oder den
// Strichcode beim Scannen — mit Werten, ohne dass wieder geschaetzt werden
// muss. Wer denselben Doener zweimal isst, bekommt so auch zweimal dieselbe
// Zahl.
type Eigenes struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Marke       string     `json:"marke,omitempty"`
	Barcode     string     `json:"barcode,omitempty"`
	Einheit     string     `json:"einheit"`
	Je100       Naehrwerte `json:"je100"`
	Sicherheit  string     `json:"sicherheit,omitempty"`
	Begruendung string     `json:"begruendung,omitempty"`
	Stand       string     `json:"stand"` // wann zuletzt geschaetzt
}

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
	Eigene    []Eigenes `json:"eigene"`
	// Einkauf ist die Einkaufsliste — der eine Teil, der sich aendert und der
	// vom Rechner zum Handy laeuft statt umgekehrt (siehe einkauf.go).
	Einkauf Einkauf `json:"einkauf"`
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
	// Ein "nachgeschlagener" Eintrag ohne Kilokalorien kam von einem neuen
	// Handy an einen alten Rechner, der "offen" noch nicht kannte und "off"
	// daraus machte. Aus Open Food Facts kommt so etwas nie — die Suche am
	// Handy laesst Produkte ohne Energie gar nicht durch. Also ist er offen.
	for i := range b.Eintraege {
		if e := &b.Eintraege[i]; e.Quelle == QuelleOFF && e.Je100.Kcal <= 0 {
			e.Quelle = QuelleOffen
		}
	}
	if b.Einkauf.Posten == nil {
		b.Einkauf.Posten = []Posten{}
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
	if e.Je100.Kcal < 0 {
		return errors.New("Kilokalorien duerfen nicht negativ sein")
	}

	// Die Quelle bestimmt der Rechner, nicht das Handy. Selbst Eingetipptes
	// ohne Kilokalorien ist offen, gleich was dabei stand — sonst zaehlte es
	// still als null. Und ein Handy kann nicht behaupten, Claude haette schon
	// geschaetzt: das passiert nur hier.
	switch {
	case e.Je100.Kcal <= 0 && e.Quelle != QuelleOFF && e.Quelle != "":
		e.Quelle = QuelleOffen
	case e.Quelle == QuelleOffen:
		e.Quelle = QuelleHand
	case e.Quelle == QuelleHand || e.Quelle == QuelleEigen:
	default:
		e.Quelle = QuelleOFF
	}
	e.Schaetzung = nil
	e.Vorher = nil
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

// ---------- Schaetzen ----------

// Offene sind die Eintraege, denen noch Naehrwerte fehlen — aelteste zuerst.
// Als Kopie: der Schaetzer arbeitet minutenlang damit, und das Buch darf sich
// in der Zeit weiter aendern.
func (b *Buch) Offene() []Eintrag {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	var liste []Eintrag
	for _, e := range b.Eintraege {
		if e.Quelle == QuelleOffen {
			liste = append(liste, e)
		}
	}
	return liste
}

// Bekannte sind Produkte, deren Werte schon feststehen — nachgeschlagen oder
// frueher geschaetzt, das Neueste zuerst. Selbst Eingetipptes zaehlt nicht
// dazu: dort steht mitunter eine 1, nur weil ein Feld nicht leer bleiben
// durfte, und die soll keine neue Schaetzung anstecken.
func (b *Buch) Bekannte(hoechstens int) []Eintrag {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	gesehen := map[string]bool{}
	var liste []Eintrag
	for i := len(b.Eintraege) - 1; i >= 0 && len(liste) < hoechstens; i-- {
		e := b.Eintraege[i]
		if (e.Quelle != QuelleOFF && e.Quelle != QuelleKI && e.Quelle != QuelleEigen) || e.Je100.Kcal <= 0 {
			continue
		}
		schluessel := strings.ToLower(e.Name + "|" + e.Marke)
		if gesehen[schluessel] {
			continue
		}
		gesehen[schluessel] = true
		liste = append(liste, e)
	}
	return liste
}

// Freigeben macht einen Eintrag wieder offen, damit Claude ihn schaetzt —
// fuer das, was mit einer Verlegenheitszahl am Handy eingetragen wurde.
//
// Die alten Werte bleiben unter Vorher liegen. Nur beim ersten Mal: wer eine
// Schaetzung ein zweites Mal verwirft, will die urspruengliche Eingabe
// nachlesen koennen und nicht die erste Schaetzung.
func (b *Buch) Freigeben(id string) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i := range b.Eintraege {
		e := &b.Eintraege[i]
		if e.ID != id {
			continue
		}
		switch e.Quelle {
		case QuelleOffen:
			return nil
		case QuelleOFF:
			return errors.New("die Werte kommen aus Open Food Facts — da gibt es nichts zu schätzen")
		}
		if e.Vorher == nil && e.Quelle == QuelleHand {
			e.Vorher = &Vorher{Quelle: e.Quelle, Je100: e.Je100}
		}
		if e.Schaetzung != nil && e.Schaetzung.NameVorher != "" {
			e.Name = e.Schaetzung.NameVorher
		}
		e.Je100 = Naehrwerte{}
		e.Quelle = QuelleOffen
		e.Schaetzung = nil
		return b.sichern()
	}
	return errors.New("kein Eintrag mit dieser ID")
}

// Schaetzen traegt Claudes Werte in einen offenen Eintrag ein.
//
// Was am Handy schon angegeben war, bleibt stehen: eine Null heisst dort
// "weiss ich nicht", alles andere hat ein Mensch hingeschrieben. Ist der
// Eintrag inzwischen geloescht oder nicht mehr offen, passiert nichts — die
// Schaetzung kam dann schlicht zu spaet.
func (b *Buch) Schaetzen(id string, je100 Naehrwerte, name string, s Schaetzung) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i := range b.Eintraege {
		e := &b.Eintraege[i]
		if e.ID != id {
			continue
		}
		if e.Quelle != QuelleOffen {
			return errors.New("der Eintrag ist nicht mehr offen")
		}
		if je100.Kcal <= 0 {
			return errors.New("Schätzung ohne Kilokalorien")
		}
		e.Je100 = behalte(e.Je100, je100)
		e.Je100.KJ = e.Je100.Kcal * 4.184
		name = strings.TrimSpace(name)
		if name != "" && name != e.Name {
			s.NameVorher = e.Name
			e.Name = name
		}
		e.Quelle = QuelleKI
		e.Schaetzung = &s
		b.merkeEigenes(*e)
		return b.sichern()
	}
	return errors.New("kein Eintrag mit dieser ID")
}

// merkeEigenes legt eine Schaetzung in der eigenen Datenbank ab. Dasselbe
// Lebensmittel — gleicher Strichcode, sonst gleicher Name und gleiche Marke —
// steht dort nur einmal, mit der juengsten Schaetzung. Aufrufer haelt die
// Sperre und sichert.
func (b *Buch) merkeEigenes(e Eintrag) {
	neu := Eigenes{
		ID: "e-" + e.ID, Name: e.Name, Marke: e.Marke, Barcode: e.Barcode,
		Einheit: e.Einheit, Je100: e.Je100, Stand: e.Schaetzung.Zeit,
		Sicherheit: e.Schaetzung.Sicherheit, Begruendung: e.Schaetzung.Begruendung,
	}
	for i, alt := range b.Eigene {
		gleich := alt.Barcode != "" && alt.Barcode == neu.Barcode
		if alt.Barcode == "" || neu.Barcode == "" {
			gleich = strings.EqualFold(alt.Name, neu.Name) && strings.EqualFold(alt.Marke, neu.Marke)
		}
		if gleich {
			neu.ID = alt.ID
			if neu.Barcode == "" {
				neu.Barcode = alt.Barcode
			}
			b.Eigene[i] = neu
			return
		}
	}
	b.Eigene = append(b.Eigene, neu)
}

// EigenesLoeschen nimmt ein Lebensmittel aus der eigenen Datenbank. Die
// Eintraege, die damit gezaehlt wurden, behalten ihre Werte.
func (b *Buch) EigenesLoeschen(id string) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i, e := range b.Eigene {
		if e.ID == id {
			b.Eigene = append(b.Eigene[:i], b.Eigene[i+1:]...)
			return b.sichern()
		}
	}
	return errors.New("kein Lebensmittel mit dieser ID")
}

// AbgleichTage ist, wie weit der Abgleich zurueckreicht — so weit, wie das
// Handy Eintraege aufhebt, und einen Tag Luft.
const AbgleichTage = 8

// Abgleich ist, was das Handy vom Rechner zurueckbekommt.
type Abgleich struct {
	Ziel   int       `json:"ziel"`
	Eigene []Eigenes `json:"eigene"`
	// Einkauf ist die Liste, wie sie hier steht. Sie geht als Ganzes mit —
	// eine Einkaufsliste ist kurz, und ein Teilabgleich waere die Stelle, an
	// der im Laden ein Posten fehlt, den jemand am Rechner dazugeschrieben hat.
	Einkauf Einkauf `json:"einkauf"`
	// Einheiten und Abteilungen gehen mit, damit das Handy sie nicht zweimal
	// kennt: das Plus dort gibt dann denselben Schritt wie das Plus hier.
	Einheiten []Einheit `json:"einheiten"`
	Gruppen   []string  `json:"gruppen"`
	// Nachtraege sind die Eintraege der letzten Tage, deren Werte hier
	// entstanden sind oder wieder offen stehen. Das Handy uebernimmt sie fuer
	// die IDs, die es noch hat — damit auch dort die Zahl von heute stimmt.
	Nachtraege []Nachtrag `json:"nachtraege"`
}

// Nachtrag sind die Werte eines Eintrags, wie sie jetzt auf dem Rechner stehen.
type Nachtrag struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Je100  Naehrwerte `json:"je100"`
	Quelle string     `json:"quelle"`
}

// Abgleichen baut den Abgleich fuers Handy.
func (b *Buch) Abgleichen() Abgleich {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	a := Abgleich{
		Ziel: b.Ziel, Eigene: append([]Eigenes{}, b.Eigene...),
		Einkauf: b.einkaufKopie(), Einheiten: Einheiten, Gruppen: Gruppen,
		Nachtraege: []Nachtrag{},
	}
	grenze := time.Now().AddDate(0, 0, -AbgleichTage).Format("2006-01-02")
	for _, e := range b.Eintraege {
		if e.Tag < grenze || (e.Quelle != QuelleKI && e.Quelle != QuelleOffen) {
			continue
		}
		a.Nachtraege = append(a.Nachtraege, Nachtrag{ID: e.ID, Name: e.Name, Je100: e.Je100, Quelle: e.Quelle})
	}
	return a
}

// behalte nimmt je Feld die angegebene Zahl und nur fuer die Nullen die
// geschaetzte.
func behalte(da, neu Naehrwerte) Naehrwerte {
	w := func(a, b float64) float64 {
		if a > 0 {
			return a
		}
		return b
	}
	return Naehrwerte{
		Kcal: w(da.Kcal, neu.Kcal), KJ: w(da.KJ, neu.KJ),
		Fett: w(da.Fett, neu.Fett), Gesaettigt: w(da.Gesaettigt, neu.Gesaettigt),
		Kohlenhydrate: w(da.Kohlenhydrate, neu.Kohlenhydrate), Zucker: w(da.Zucker, neu.Zucker),
		Ballaststoffe: w(da.Ballaststoffe, neu.Ballaststoffe), Eiweiss: w(da.Eiweiss, neu.Eiweiss),
		Salz: w(da.Salz, neu.Salz),
	}
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
