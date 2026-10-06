package tagebuch

// Die Einkaufsliste laeuft in die andere Richtung als der Rest.
//
// Alles sonst entsteht am Handy und wird hier gesammelt: der Eintrag ist ein
// Ereignis, und er ist unveraenderlich. Die Liste entsteht hier — Claude
// schreibt sie aus dem Tagebuch — und wird dort gebraucht, im Laden, wo der
// Rechner nicht steht.
//
// Deshalb ist sie auch der eine Teil des Tagebuchs, der sich aendert. Einen
// Posten abzuhaken oder die Menge hochzudruecken ist keine
// Geschichtsfaelschung, sondern der Zweck: eine Liste, die nicht abnimmt, ist
// keine.
//
// Hier stehen Mengen und keine Preise. Was etwas kostet, weiss dieses Programm
// nicht: es kennt das Tagebuch, nicht die Regale. Eine Zahl, die wie ein Preis
// aussieht und geraten ist, waere schlechter als keine.
//
// Geaendert wird auf beiden Seiten, und deshalb traegt jede Zeile die Zeit
// ihrer letzten Aenderung. Wer zuletzt gedrueckt hat, gewinnt — nicht, wessen
// Nachricht zuletzt ankam. Sonst setzte ein Handy, das eine Stunde ohne Netz
// im Laden stand, beim Nachschicken einen Haken zurueck, den am Rechner
// inzwischen jemand gesetzt hat.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Woher ein Posten kommt.
const (
	VonKI   = "ki"   // von Claude aus dem Tagebuch geschrieben
	VonHand = "hand" // am Rechner oder am Handy selbst dazugeschrieben
)

// Gruppen sind die Abteilungen, in der Reihenfolge, in der man einen Laden
// durchgeht. Eine feste Liste und keine freie Eingabe: sonst stehen nach drei
// Listen „Gemuese", „Obst & Gemüse" und „Frischware" untereinander, und die
// Gruppierung sortiert nichts mehr zusammen.
var Gruppen = []string{
	"Obst & Gemüse",
	"Brot & Backwaren",
	"Kühlregal",
	"Fleisch & Fisch",
	"Trockenwaren",
	"Konserven & Gläser",
	"Tiefkühl",
	"Getränke",
	"Süßes & Snacks",
	"Haushalt",
	"Sonstiges",
}

// GruppeSonstiges ist die Auffangabteilung — und die Vorgabe fuer alles, was
// sich keiner zuordnen laesst.
const GruppeSonstiges = "Sonstiges"

// gruppenPlatz sagt, an welcher Stelle im Laden eine Abteilung liegt.
var gruppenPlatz = func() map[string]int {
	m := make(map[string]int, len(Gruppen))
	for i, g := range Gruppen {
		m[g] = i
	}
	return m
}()

// Einheit ist eine Mengeneinheit samt Schrittweite.
//
// Die Schrittweite steht hier und nicht in der Oberflaeche: das Plus soll auf
// beiden Seiten dasselbe tun, und 100 g weiter ist bei Mehl richtig, bei
// Eiern waere es Unfug. Deshalb gibt jede Einheit ihren eigenen Schritt vor.
type Einheit struct {
	Name    string  `json:"name"`    // wie sie gespeichert wird
	Eins    string  `json:"eins"`    // „Packung"
	Viele   string  `json:"viele"`   // „Packungen"
	Schritt float64 `json:"schritt"` // was ein Druck auf + dazugibt
}

// Einheiten sind die, die auf einem Einkaufszettel stehen. Absichtlich wenige:
// jede weitere ist eine Entscheidung mehr beim Eintragen, und „2 Kartons
// Milch" kauft niemand anders ein als „2 Packungen".
var Einheiten = []Einheit{
	{"stueck", "Stück", "Stück", 1},
	{"g", "g", "g", 50},
	{"kg", "kg", "kg", 0.5},
	{"ml", "ml", "ml", 100},
	{"l", "l", "l", 0.5},
	{"packung", "Packung", "Packungen", 1},
	{"beutel", "Beutel", "Beutel", 1},
	{"dose", "Dose", "Dosen", 1},
	{"glas", "Glas", "Gläser", 1},
	{"becher", "Becher", "Becher", 1},
	{"flasche", "Flasche", "Flaschen", 1},
	{"bund", "Bund", "Bund", 1},
}

// EinheitVorgabe gilt, wenn keine oder eine unbekannte dastand.
const EinheitVorgabe = "stueck"

// EinheitZu findet eine Einheit ueber ihren Namen oder ihre Beschriftung.
// Claude schreibt „Packung", gespeichert wird „packung", und beides soll
// hier ankommen.
func EinheitZu(name string) Einheit {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimSuffix(name, ".")
	for _, e := range Einheiten {
		if name == e.Name || name == strings.ToLower(e.Eins) || name == strings.ToLower(e.Viele) {
			return e
		}
	}
	// Ein paar Schreibweisen, die oefter kommen als die gespeicherten.
	switch name {
	case "stück", "stueck", "st", "stk", "x", "":
		return Einheiten[0]
	case "gramm":
		return Einheiten[1]
	case "kilo", "kilogramm":
		return Einheiten[2]
	case "milliliter":
		return Einheiten[3]
	case "liter":
		return Einheiten[4]
	case "pack", "pck", "päckchen", "karton":
		return Einheiten[5]
	case "tüte", "sack":
		return Einheiten[6]
	}
	return Einheiten[0]
}

// Posten ist eine Zeile auf der Einkaufsliste.
type Posten struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Menge   float64 `json:"menge"`
	Einheit string  `json:"einheit"`
	Gruppe  string  `json:"gruppe"`
	// Grund ist der halbe Satz, warum der Posten auf der Liste steht — von
	// Claude, damit eine Liste nachvollziehbar bleibt, ohne dass jemand
	// nachfragen muss.
	Grund string `json:"grund,omitempty"`
	Von   string `json:"von"`

	Erledigt bool `json:"erledigt"`
	// Geaendert ist die Zeit der letzten Aenderung an dieser Zeile — Haken
	// wie Menge. Daran entscheidet sich beim Abgleich mit dem Handy, welche
	// von zwei Fassungen gilt.
	Geaendert string `json:"geaendert,omitempty"`
	Wer       string `json:"wer,omitempty"` // Geraete-ID; leer heisst: an diesem Rechner
	Angelegt  string `json:"angelegt"`
}

// MengeText schreibt die Menge so, wie sie auf einem Zettel stehen wuerde.
func (p Posten) MengeText() string {
	e := EinheitZu(p.Einheit)
	name := e.Viele
	if math.Abs(p.Menge-1) < 0.001 {
		name = e.Eins
	}
	return zahlKurz(p.Menge) + " " + name
}

// Einkauf ist die eine Liste. Es gibt nicht mehrere: wer einkaufen geht, geht
// mit einer Liste los, und eine zweite daneben waere die, die man vergisst.
type Einkauf struct {
	Posten []Posten `json:"posten"`

	// Wunsch ist, was beim Erstellen verlangt wurde, Notiz der Satz, den
	// Claude dazu geschrieben hat. Beides steht hier, damit im Fenster
	// nachlesbar bleibt, woraus diese Liste entstanden ist.
	Wunsch   string `json:"wunsch,omitempty"`
	Notiz    string `json:"notiz,omitempty"`
	Modell   string `json:"modell,omitempty"`
	Erstellt string `json:"erstellt,omitempty"`

	// Stand ist, wann sich hier zuletzt etwas geaendert hat. Das Handy
	// vergleicht damit und schreibt seine Abschrift nur bei Bedarf neu.
	Stand string `json:"stand,omitempty"`
}

// Offen zaehlt, was noch zu holen ist.
func (e Einkauf) Offen() int {
	n := 0
	for _, p := range e.Posten {
		if !p.Erledigt {
			n++
		}
	}
	return n
}

// HoechstensPosten deckelt die Liste. Was darueber liegt, ist kein Einkauf
// mehr, sondern ein Lagerbestand — und im Laden liest das niemand.
const HoechstensPosten = 120

// HoechsteMenge ist die Grenze nach oben. Wer 200 Packungen braucht, tippt
// keinen Einkaufszettel, sondern eine Bestellung.
const HoechsteMenge = 10000

// ---------- Lesen ----------

// EinkaufHolen gibt die Liste heraus, nach Abteilungen geordnet. Als Kopie:
// die Oberflaeche zeichnet daraus, und das Buch darf sich in der Zeit weiter
// aendern.
func (b *Buch) EinkaufHolen() Einkauf {
	b.sperre.Lock()
	defer b.sperre.Unlock()
	return b.einkaufKopie()
}

// einkaufKopie sortiert nach Abteilung und laesst die Reihenfolge darin, wie
// sie ist. Aufrufer haelt die Sperre.
//
// Abgehaktes bleibt an seinem Platz: eine Liste, die beim Abhaken die Zeilen
// umsortiert, verliert genau den Halt, den man im Laden braucht.
func (b *Buch) einkaufKopie() Einkauf {
	e := b.Einkauf
	e.Posten = append(make([]Posten, 0, len(b.Einkauf.Posten)), b.Einkauf.Posten...)
	sort.SliceStable(e.Posten, func(i, j int) bool {
		return gruppenPlatz[e.Posten[i].Gruppe] < gruppenPlatz[e.Posten[j].Gruppe]
	})
	return e
}

// ---------- Schreiben ----------

// EinkaufSchreiben legt ab, was Claude vorgeschlagen hat.
//
// ersetzen wirft die alte Liste weg; sonst kommt dazu, was noch nicht
// draufsteht. Die Dopplung wird hier abgefangen und nicht Claude
// ueberlassen — er sieht die Liste zwar, aber zweimal Milch im Wagen ist
// schlimmer als eine Zeile weniger.
func (b *Buch) EinkaufSchreiben(neu []Posten, ersetzen bool, wunsch, notiz, modell string) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	jetzt := time.Now().Format(time.RFC3339)
	alt := b.Einkauf.Posten
	if ersetzen {
		alt = nil
	}
	da := map[string]bool{}
	for _, p := range alt {
		da[schluessel(p.Name)] = true
	}

	liste := append([]Posten{}, alt...)
	dazu := 0
	for _, p := range neu {
		p.Von = VonKI
		p.Erledigt = false
		if err := pruefePosten(&p, jetzt); err != nil {
			continue // eine unbrauchbare Zeile ist kein Grund, die Liste zu verwerfen
		}
		if da[schluessel(p.Name)] || len(liste) >= HoechstensPosten {
			continue
		}
		da[schluessel(p.Name)] = true
		liste = append(liste, p)
		dazu++
	}
	if dazu == 0 {
		if ersetzen {
			return errors.New("kein brauchbarer Posten dabei")
		}
		return errors.New("nichts Neues dabei — alles davon steht schon auf der Liste")
	}

	b.Einkauf.Posten = liste
	b.Einkauf.Wunsch = kurz(wunsch, 400)
	b.Einkauf.Notiz = kurz(notiz, 600)
	b.Einkauf.Modell = modell
	b.Einkauf.Erstellt = jetzt
	b.Einkauf.Stand = jetzt
	return b.sichern()
}

// PostenAnlegen schreibt einen Posten von Hand dazu.
func (b *Buch) PostenAnlegen(p Posten) (Posten, error) {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	p.Von = VonHand
	p.Erledigt = false
	if err := pruefePosten(&p, time.Now().Format(time.RFC3339)); err != nil {
		return Posten{}, err
	}
	if len(b.Einkauf.Posten) >= HoechstensPosten {
		return Posten{}, errors.New("die Liste ist voll")
	}
	for _, alt := range b.Einkauf.Posten {
		if schluessel(alt.Name) == schluessel(p.Name) {
			return Posten{}, errors.New("„" + p.Name + "“ steht schon auf der Liste")
		}
	}
	b.Einkauf.Posten = append(b.Einkauf.Posten, p)
	b.Einkauf.Stand = p.Angelegt
	return p, b.sichern()
}

// PostenAufnehmen nimmt Posten vom Handy an — im Laden fehlt einem etwas ein,
// und dann soll es auf die Liste, auf der es gebraucht wird.
//
// Ueber die ID und damit gefahrlos zweimal: dieselbe Warteschlangenlogik wie
// bei den Eintraegen. Zurueck kommt, wie viele neu waren.
func (b *Buch) PostenAufnehmen(neu []Posten, geraet string) (int, error) {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	jetzt := time.Now().Format(time.RFC3339)
	da := map[string]bool{}
	for _, p := range b.Einkauf.Posten {
		da[p.ID] = true
	}

	gezaehlt := 0
	for _, p := range neu {
		if p.ID == "" || da[p.ID] || len(b.Einkauf.Posten) >= HoechstensPosten {
			continue
		}
		p.Von = VonHand
		p.Wer = geraet
		if err := pruefePosten(&p, jetzt); err != nil {
			continue
		}
		da[p.ID] = true
		b.Einkauf.Posten = append(b.Einkauf.Posten, p)
		gezaehlt++
	}
	if gezaehlt == 0 {
		return 0, nil
	}
	b.Einkauf.Stand = jetzt
	return gezaehlt, b.sichern()
}

// PostenHaken setzt oder loest den Haken.
func (b *Buch) PostenHaken(id string, erledigt bool, wann, wer string) error {
	return b.postenAendern(id, wann, wer, func(p *Posten) bool {
		if p.Erledigt == erledigt {
			return false
		}
		p.Erledigt = erledigt
		return true
	})
}

// PostenSchritt gibt eine Schrittweite dazu oder nimmt sie weg — das, was die
// beiden Knoepfe + und − tun.
//
// Nach unten endet es bei einem Schritt und nicht bei null: ein Posten mit der
// Menge null steht auf der Liste und behauptet gleichzeitig, es sei nichts zu
// holen. Wer ihn nicht braucht, nimmt ihn weg.
func (b *Buch) PostenSchritt(id string, richtung int, wer string) (Posten, error) {
	var treffer Posten
	err := b.postenAendern(id, "", wer, func(p *Posten) bool {
		e := EinheitZu(p.Einheit)
		neu := p.Menge + e.Schritt*float64(sign(richtung))
		neu = math.Round(neu*1000) / 1000
		if neu < e.Schritt {
			neu = e.Schritt
		}
		if neu > HoechsteMenge {
			neu = HoechsteMenge
		}
		geaendert := neu != p.Menge
		p.Menge = neu
		treffer = *p
		return geaendert
	})
	return treffer, err
}

// PostenMenge setzt die Menge direkt — der Weg fuer das Handy, das seine
// Aenderung nachschickt, und fuer eine Zahl, die jemand eintippt.
func (b *Buch) PostenMenge(id string, menge float64, einheit, wann, wer string) error {
	return b.postenAendern(id, wann, wer, func(p *Posten) bool {
		e := EinheitZu(p.Einheit)
		if einheit != "" {
			e = EinheitZu(einheit)
		}
		if menge <= 0 || menge > HoechsteMenge || math.IsNaN(menge) || math.IsInf(menge, 0) {
			return false
		}
		menge = math.Round(menge*1000) / 1000
		if menge == p.Menge && e.Name == p.Einheit {
			return false
		}
		p.Menge = menge
		p.Einheit = e.Name
		return true
	})
}

// postenAendern ist der eine Weg, auf dem sich eine Zeile aendert.
//
// wann ist die Zeit, zu der gedrueckt wurde — am Handy also die dortige, nicht
// die des Empfangs. Ein aelterer Druck ueberschreibt keinen juengeren: das
// Handy schickt nach, bis es durchkommt, und was unterwegs am Rechner
// passierte, soll dabei nicht zurueckgedreht werden.
func (b *Buch) postenAendern(id, wann, wer string, aendern func(*Posten) bool) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	if _, err := time.Parse(time.RFC3339, wann); err != nil {
		wann = time.Now().Format(time.RFC3339)
	}
	for i := range b.Einkauf.Posten {
		p := &b.Einkauf.Posten[i]
		if p.ID != id {
			continue
		}
		if p.Geaendert > wann {
			return nil // eine juengere Aenderung liegt schon
		}
		if !aendern(p) {
			return nil
		}
		p.Geaendert = wann
		p.Wer = wer
		b.Einkauf.Stand = time.Now().Format(time.RFC3339)
		return b.sichern()
	}
	return errors.New("kein Posten mit dieser ID")
}

// PostenLoeschen nimmt eine Zeile ganz weg.
func (b *Buch) PostenLoeschen(id string) error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	for i, p := range b.Einkauf.Posten {
		if p.ID == id {
			b.Einkauf.Posten = append(b.Einkauf.Posten[:i], b.Einkauf.Posten[i+1:]...)
			b.Einkauf.Stand = time.Now().Format(time.RFC3339)
			return b.sichern()
		}
	}
	return errors.New("kein Posten mit dieser ID")
}

// EinkaufAufraeumen wirft weg, was im Wagen liegt. Zurueck kommt, wie viele
// Zeilen gegangen sind.
func (b *Buch) EinkaufAufraeumen() (int, error) {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	bleibt := make([]Posten, 0, len(b.Einkauf.Posten))
	for _, p := range b.Einkauf.Posten {
		if !p.Erledigt {
			bleibt = append(bleibt, p)
		}
	}
	weg := len(b.Einkauf.Posten) - len(bleibt)
	if weg == 0 {
		return 0, nil
	}
	b.Einkauf.Posten = bleibt
	b.Einkauf.Stand = time.Now().Format(time.RFC3339)
	return weg, b.sichern()
}

// EinkaufLeeren raeumt die ganze Liste weg. Wunsch und Notiz gehen mit: sie
// beschreiben eine Liste, die es dann nicht mehr gibt.
func (b *Buch) EinkaufLeeren() error {
	b.sperre.Lock()
	defer b.sperre.Unlock()

	if len(b.Einkauf.Posten) == 0 {
		return nil
	}
	b.Einkauf = Einkauf{Posten: []Posten{}, Stand: time.Now().Format(time.RFC3339)}
	return b.sichern()
}

// ---------- Kleinkram ----------

// pruefePosten haelt Unfug draussen und fuellt auf, was sich ableiten laesst.
func pruefePosten(p *Posten, jetzt string) error {
	p.Name = einzeilig(p.Name)
	p.Grund = einzeilig(p.Grund)
	if p.Name == "" {
		return errors.New("Posten ohne Namen")
	}
	p.Name = kurz(p.Name, 80)
	p.Grund = kurz(p.Grund, 200)
	p.Gruppe = GruppeRein(p.Gruppe)

	e := EinheitZu(p.Einheit)
	p.Einheit = e.Name
	if math.IsNaN(p.Menge) || math.IsInf(p.Menge, 0) || p.Menge <= 0 {
		p.Menge = e.Schritt
	}
	if p.Menge > HoechsteMenge {
		p.Menge = HoechsteMenge
	}
	p.Menge = math.Round(p.Menge*1000) / 1000

	if p.ID == "" {
		p.ID = "p-" + zufallsID()
	}
	if p.Von != VonKI {
		p.Von = VonHand
	}
	if p.Angelegt == "" {
		p.Angelegt = jetzt
	}
	if !p.Erledigt {
		p.Geaendert = ""
	}
	return nil
}

// GruppeRein laesst nur die bekannten Abteilungen durch. Alles andere ist
// Sonstiges — eine erfundene Abteilung waere eine Abteilung, in der genau ein
// Posten steht.
func GruppeRein(g string) string {
	g = einzeilig(g)
	for _, bekannt := range Gruppen {
		if strings.EqualFold(g, bekannt) {
			return bekannt
		}
	}
	return GruppeSonstiges
}

// schluessel ist der Name, auf Dopplung hin verglichen: Gross- und
// Kleinschreibung und Randleerzeichen zaehlen dabei nicht.
func schluessel(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

// zahlKurz schreibt eine Menge ohne ueberfluessige Nullen und mit Komma:
// 2 statt 2,0 und 0,5 statt 0,500.
func zahlKurz(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return strings.Replace(s, ".", ",", 1)
}

func einzeilig(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func kurz(s string, hoechstens int) string {
	r := []rune(s)
	if len(r) <= hoechstens {
		return s
	}
	return strings.TrimSpace(string(r[:hoechstens]))
}

// zufallsID vergibt die Kennung eines Postens, der hier entsteht. Die vom
// Handy kommen mit ihrer eigenen — daran haengt, dass ein Nachschicken nichts
// doppelt anlegt.
func zufallsID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
