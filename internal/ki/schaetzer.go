package ki

// Der Schaetzer traegt Naehrwerte nach.
//
// Am Handy muss man nur wissen, was man gegessen hat und wie viel. Steht es in
// keiner Datenbank, geht der Eintrag ohne Werte heraus — und hier nimmt Claude
// sich die offenen Eintraege vor und schaetzt, was je 100 g drinsteckt. Die
// Zahlen landen mit der Quelle "ki" im Tagebuch: sie zaehlen damit in Tag,
// Woche und Raster, und man sieht ihnen trotzdem an, dass sie geschaetzt sind.
//
// Anders als im Gespraech gibt es hier keine Werkzeuge, keine Sitzung und
// keinen Mitschnitt: ein Aufruf, eine Antwort in einem festen JSON-Schema
// (`--json-schema`), und eingetragen wird vom Programm, nicht von Claude. So
// kann ein Aufruf nichts anderes tun, als Zahlen vorzuschlagen — und das
// Programm prueft sie, bevor sie zaehlen.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/1Zane0/esstracker/internal/speicher"
	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// JeDurchgang deckelt, wie viele Eintraege in einen Aufruf gehen. Kommt nach
// einer Woche ohne Rechner ein ganzer Stapel an, laufen eben mehrere
// Durchgaenge nacheinander — ein einzelner riesiger waere der, der an der
// Frist scheitert und alles mitnimmt.
const JeDurchgang = 20

// BekannteImAufruf ist, wie viele feststehende Produkte als Vergleich
// mitgehen. Wer zweimal "Doener" isst, soll zweimal dieselbe Zahl bekommen.
const BekannteImAufruf = 40

// Schaetzer arbeitet die offenen Eintraege ab, einer zur Zeit.
//
// Angestossen wird er von allein, sobald vom Handy etwas Offenes hereinkommt
// und beim Start, und von Hand ueber den Knopf. Was von allein schon einmal
// versucht wurde und nichts ergab, wird von allein nicht noch einmal
// versucht: ein nicht angemeldetes claude soll nicht bei jedem Eintrag vom
// Handy aufs Neue scheitern. Der Knopf vergisst das.
type Schaetzer struct {
	buch *tagebuch.Buch

	sperre   sync.Mutex
	laeuft   bool // die Schleife arbeitet
	nochmal  bool // waehrend sie lief, kam etwas Neues
	halt     bool // Notbremse gezogen
	seit     time.Time
	anzahl   int // Eintraege im laufenden Aufruf, 0 dazwischen
	fehler   string
	zuletzt  time.Time
	unklar   map[string]string
	versucht map[string]bool
	stoppen  context.CancelCauseFunc
}

// NeuerSchaetzer baut den Schaetzer. Er tut nichts, bis er angestossen wird.
func NeuerSchaetzer(b *tagebuch.Buch) *Schaetzer {
	return &Schaetzer{buch: b, unklar: map[string]string{}, versucht: map[string]bool{}}
}

// Anstossen laesst die offenen Eintraege schaetzen. vonHand heisst: ein Mensch
// hat gedrueckt, also auch das noch einmal versuchen, was vorhin scheiterte.
func (s *Schaetzer) Anstossen(vonHand bool) {
	s.sperre.Lock()
	defer s.sperre.Unlock()

	if vonHand {
		s.versucht = map[string]bool{}
		s.unklar = map[string]string{}
		s.fehler = ""
	}
	if s.laeuft {
		s.nochmal = true
		return
	}
	s.laeuft = true
	s.halt = false
	go s.schleife()
}

// Abbrechen stoppt den laufenden Aufruf. Was schon eingetragen ist, bleibt.
func (s *Schaetzer) Abbrechen() error {
	s.sperre.Lock()
	defer s.sperre.Unlock()

	if !s.laeuft {
		return errors.New("da wird gerade nichts geschätzt")
	}
	s.halt = true
	s.nochmal = false
	if s.stoppen != nil {
		s.stoppen(errAbbruch)
	}
	return nil
}

// Stand gibt heraus, was die Oberflaeche zeigt.
func (s *Schaetzer) Stand() tagebuch.SchaetzStand {
	bereit, hinweis := Bereit()

	s.sperre.Lock()
	defer s.sperre.Unlock()

	st := tagebuch.SchaetzStand{
		Bereit: bereit, Hinweis: hinweis,
		Laeuft: s.laeuft, Anzahl: s.anzahl,
		Fehler: s.fehler, Unklar: map[string]string{},
	}
	if !s.seit.IsZero() {
		st.Seit = s.seit.Format(time.RFC3339)
	}
	if !s.zuletzt.IsZero() {
		st.Zuletzt = s.zuletzt.Format(time.RFC3339)
	}
	for id, grund := range s.unklar {
		st.Unklar[id] = grund
	}
	return st
}

func (s *Schaetzer) schleife() {
	for {
		faellig := s.faellige()

		s.sperre.Lock()
		if s.halt || len(faellig) == 0 {
			if !s.halt && s.nochmal {
				s.nochmal = false
				s.sperre.Unlock()
				continue
			}
			s.laeuft, s.halt, s.nochmal = false, false, false
			s.sperre.Unlock()
			return
		}
		s.sperre.Unlock()

		s.durchgang(faellig)
	}
}

// faellige sind die offenen Eintraege, die in diesem Anlauf noch nicht
// versucht wurden — hoechstens so viele, wie in einen Aufruf passen.
func (s *Schaetzer) faellige() []tagebuch.Eintrag {
	offen := s.buch.Offene()

	s.sperre.Lock()
	defer s.sperre.Unlock()

	var liste []tagebuch.Eintrag
	for _, e := range offen {
		if s.versucht[e.ID] {
			continue
		}
		liste = append(liste, e)
		if len(liste) == JeDurchgang {
			break
		}
	}
	return liste
}

func (s *Schaetzer) durchgang(faellig []tagebuch.Eintrag) {
	frist, endeFrist := context.WithTimeout(context.Background(), Frist)
	defer endeFrist()
	ctx, stopp := context.WithCancelCause(frist)
	defer stopp(nil)

	s.sperre.Lock()
	s.seit, s.anzahl, s.stoppen = time.Now(), len(faellig), stopp
	s.sperre.Unlock()

	fehler := ""
	unklar := map[string]string{}

	if bereit, warum := Bereit(); !bereit {
		fehler = warum
	} else if antwort, modell, err := schaetze(ctx, faellig, s.buch.Bekannte(BekannteImAufruf)); err != nil {
		fehler = err.Error()
	} else {
		unklar = s.eintragen(faellig, antwort, modell)
	}

	s.sperre.Lock()
	// Alles aus diesem Durchgang gilt als versucht. Was geschaetzt wurde, ist
	// ohnehin nicht mehr offen; der Rest soll die Schleife nicht endlos
	// drehen lassen.
	for _, e := range faellig {
		s.versucht[e.ID] = true
	}
	for id, grund := range unklar {
		s.unklar[id] = grund
	}
	s.fehler = fehler
	s.zuletzt = time.Now()
	s.seit, s.anzahl, s.stoppen = time.Time{}, 0, nil
	s.sperre.Unlock()
}

// eintragen prueft jede Schaetzung und traegt ein, was taugt. Zurueck kommt,
// was offen bleibt, samt Grund.
func (s *Schaetzer) eintragen(faellig []tagebuch.Eintrag, a antwort, modell string) map[string]string {
	offen := map[string]bool{}
	for _, e := range faellig {
		offen[e.ID] = true
	}
	unklar := map[string]string{}
	jetzt := time.Now().Format(time.RFC3339)

	for _, v := range a.Eintraege {
		if !offen[v.ID] {
			continue // eine ID, nach der niemand gefragt hat
		}
		delete(offen, v.ID)
		werte, warum := v.pruefe()
		if warum != "" {
			unklar[v.ID] = warum
			continue
		}
		// Scheitert das, wurde der Eintrag inzwischen geloescht oder anders
		// erledigt. Dann gibt es auch nichts mehr zu melden.
		_ = s.buch.Schaetzen(v.ID, werte, v.Name, tagebuch.Schaetzung{
			Zeit: jetzt, Modell: modell,
			Sicherheit: v.Sicherheit, Begruendung: strings.TrimSpace(v.Begruendung),
		})
	}
	for _, u := range a.Unklar {
		if offen[u.ID] {
			delete(offen, u.ID)
			unklar[u.ID] = strings.TrimSpace(u.Grund)
		}
	}
	for id := range offen {
		unklar[id] = "Claude hat zu diesem Eintrag nichts gesagt."
	}
	return unklar
}

// ---------- Claude ----------

// schaetzAnweisung ist der ganze Systemtext — nicht angehaengt an den von
// Claude Code, sondern an seiner Stelle. Dort steht, wie man programmiert;
// hier geht es um Heidelbeeren.
const schaetzAnweisung = `Du schätzt Nährwerte für ein Essenstagebuch. Am Handy wurde nur eingetragen,
was gegessen wurde und wie viel — das Essen stand in keiner Datenbank. Deine
Zahlen gehen ohne Rückfrage ins Tagebuch und zählen dort mit. Sie sind dort als
geschätzt gekennzeichnet.

Für jeden Eintrag in <offen>:

- Werte je 100 g — bei der Einheit "ml" je 100 ml —, nie für die gegessene
  Menge. kcal in Kilokalorien, alles andere in Gramm: fett, gesaettigt
  (gesättigte Fettsäuren), kohlenhydrate, zucker, ballaststoffe, eiweiss, salz.
- Richte dich nach üblichen Nährwerttabellen (Bundeslebensmittelschlüssel,
  USDA) und nimm die Form, in der man es isst: gekocht, wenn man es gekocht
  isst. Bei einem Gericht die typische Zubereitung in Deutschland. Steht eine
  Marke oder ein Ort dabei, nimm, was dort typisch ist.
- Steht in <bekannt> dasselbe Lebensmittel, übernimm diese Werte.
- Werte unter "angegeben" hat der Mensch selbst eingetragen: übernimm sie
  unverändert und schätze die übrigen passend dazu.
- name: derselbe Name, nur sauber geschrieben — Rechtschreibung, und
  Substantive groß, wie im Deutschen üblich: aus "döner mit soße" wird "Döner
  mit Soße", aus "brokollistamm" "Brokkolistamm". Nichts ergänzen, nichts
  übersetzen, keine Marke dazu.
- sicherheit: "hoch" für einfache Lebensmittel mit festen Tabellenwerten,
  "mittel" für Gerichte, deren Rezept schwankt, "niedrig", wenn der Name
  mehreres meinen kann.
- begruendung: ein kurzer Satz auf Deutsch, worauf die Zahl beruht, etwa
  "Tabellenwert für rohe Heidelbeeren" oder "Rindergulasch mit Soße, ohne
  Beilage".

Die Menge steht fest und wird nicht geschätzt. Sie hilft nur beim Verstehen:
400 g "Pizza" ist eine ganze, 30 g "Nüsse" eine Handvoll.

Lässt sich beim besten Willen nicht sagen, was gemeint ist — Buchstabensalat,
ein Name wie "Zeug" —, dann schätze nicht, sondern setze den Eintrag unter
"unklar", mit einem kurzen Grund. Jede ID kommt genau einmal vor: entweder in
"eintraege" oder in "unklar".`

// schema legt die Antwort fest. Die CLI prueft sie, bevor sie zurueckkommt —
// ein Satz statt einer Zahl kann hier also gar nicht erst ankommen.
const schema = `{
  "type": "object",
  "properties": {
    "eintraege": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "name": {"type": "string"},
          "kcal": {"type": "number"},
          "fett": {"type": "number"},
          "gesaettigt": {"type": "number"},
          "kohlenhydrate": {"type": "number"},
          "zucker": {"type": "number"},
          "ballaststoffe": {"type": "number"},
          "eiweiss": {"type": "number"},
          "salz": {"type": "number"},
          "sicherheit": {"type": "string", "enum": ["hoch", "mittel", "niedrig"]},
          "begruendung": {"type": "string"}
        },
        "required": ["id", "name", "kcal", "fett", "gesaettigt", "kohlenhydrate", "zucker", "ballaststoffe", "eiweiss", "salz", "sicherheit", "begruendung"],
        "additionalProperties": false
      }
    },
    "unklar": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": {"type": "string"},
          "grund": {"type": "string"}
        },
        "required": ["id", "grund"],
        "additionalProperties": false
      }
    }
  },
  "required": ["eintraege", "unklar"],
  "additionalProperties": false
}`

type antwort struct {
	Eintraege []vorschlag `json:"eintraege"`
	Unklar    []struct {
		ID    string `json:"id"`
		Grund string `json:"grund"`
	} `json:"unklar"`
}

type vorschlag struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Kcal          float64 `json:"kcal"`
	Fett          float64 `json:"fett"`
	Gesaettigt    float64 `json:"gesaettigt"`
	Kohlenhydrate float64 `json:"kohlenhydrate"`
	Zucker        float64 `json:"zucker"`
	Ballaststoffe float64 `json:"ballaststoffe"`
	Eiweiss       float64 `json:"eiweiss"`
	Salz          float64 `json:"salz"`
	Sicherheit    string  `json:"sicherheit"`
	Begruendung   string  `json:"begruendung"`
}

// pruefe haelt fest, was physikalisch nicht geht. Mehr als 900 kcal hat nicht
// einmal reines Fett, und in 100 g stecken keine 140 g Naehrstoffe. Eine
// solche Zahl ist keine grobe Schaetzung mehr, sondern ein Fehler — sie zaehlt
// nicht, der Eintrag bleibt offen.
func (v vorschlag) pruefe() (tagebuch.Naehrwerte, string) {
	n := tagebuch.Naehrwerte{
		Kcal: v.Kcal, Fett: v.Fett, Gesaettigt: v.Gesaettigt,
		Kohlenhydrate: v.Kohlenhydrate, Zucker: v.Zucker,
		Ballaststoffe: v.Ballaststoffe, Eiweiss: v.Eiweiss, Salz: v.Salz,
	}
	for _, w := range []float64{n.Kcal, n.Fett, n.Gesaettigt, n.Kohlenhydrate, n.Zucker, n.Ballaststoffe, n.Eiweiss, n.Salz} {
		if math.IsNaN(w) || math.IsInf(w, 0) || w < 0 {
			return n, "Claude hat eine unbrauchbare Zahl geliefert."
		}
	}
	if n.Kcal <= 0 || n.Kcal > 900 {
		return n, fmt.Sprintf("Unplausible Schätzung: %.0f kcal je 100 g.", n.Kcal)
	}
	if n.Fett+n.Kohlenhydrate+n.Eiweiss+n.Ballaststoffe+n.Salz > 105 {
		return n, "Unplausible Schätzung: mehr als 100 g Nährstoffe in 100 g."
	}
	// Ein Teil kann nicht groesser sein als das Ganze. Das ist eine
	// Rundungsfrage und kein Grund, die ganze Schaetzung zu verwerfen.
	n.Gesaettigt = math.Min(n.Gesaettigt, n.Fett)
	n.Zucker = math.Min(n.Zucker, n.Kohlenhydrate)
	return n, ""
}

// offenerEintrag ist das, was Claude von einem Eintrag zu sehen bekommt:
// genug zum Verstehen, keine Geraete-IDs und keine Uhrzeiten des Empfangs.
type offenerEintrag struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Marke     string             `json:"marke,omitempty"`
	Barcode   string             `json:"barcode,omitempty"`
	Menge     float64            `json:"menge"`
	Einheit   string             `json:"einheit"`
	Mahlzeit  string             `json:"mahlzeit"`
	Angegeben map[string]float64 `json:"angegeben,omitempty"`
}

func schaetze(ctx context.Context, faellig []tagebuch.Eintrag, bekannte []tagebuch.Eintrag) (antwort, string, error) {
	ordner := speicher.Ordner()
	if err := os.MkdirAll(ordner, 0o700); err != nil {
		return antwort{}, "", fmt.Errorf("datenordner anlegen: %w", err)
	}

	liste := make([]offenerEintrag, 0, len(faellig))
	for _, e := range faellig {
		liste = append(liste, offenerEintrag{
			ID: e.ID, Name: e.Name, Marke: e.Marke, Barcode: e.Barcode,
			Menge: e.Menge, Einheit: e.Einheit, Mahlzeit: mahlzeit(e.Mahlzeit),
			Angegeben: angegeben(e.Je100),
		})
	}
	roh, err := json.MarshalIndent(liste, "", "  ")
	if err != nil {
		return antwort{}, "", err
	}

	var text strings.Builder
	text.WriteString("<offen>\n")
	text.Write(roh)
	text.WriteString("\n</offen>\n\n<bekannt>\n")
	if len(bekannte) == 0 {
		text.WriteString("Noch nichts.\n")
	}
	for _, e := range bekannte {
		n := e.Je100
		einheit := e.Einheit
		if einheit == "" {
			einheit = "g"
		}
		fmt.Fprintf(&text, "- %s (%s), je 100 %s: %s kcal, Fett %s, davon gesättigt %s, Kohlenhydrate %s, davon Zucker %s, Ballaststoffe %s, Eiweiß %s, Salz %s\n",
			feld(e.Name), feld(e.Marke), einheit, gr(n.Kcal), gr(n.Fett), gr(n.Gesaettigt),
			gr(n.Kohlenhydrate), gr(n.Zucker), gr(n.Ballaststoffe), gr(n.Eiweiss), gr(n.Salz))
	}
	text.WriteString("</bekannt>")

	argumente := []string{
		"-p", "--output-format", "json",
		"--model", Modell,
		"--effort", Aufwand,
		"--system-prompt", schaetzAnweisung,
		// Nichts aufheben: eine Schaetzung ist kein Gespraech, und die
		// Sitzungsliste der CLI soll nicht mit Heidelbeeren volllaufen.
		"--no-session-persistence",
		// Keine Werkzeuge, auch keine von MCP-Servern aus der Konfiguration.
		// Hier wird nur geantwortet.
		"--strict-mcp-config",
		"--tools", "",
		// Das Schema steht als letzter Schalter vor dem Text: "--tools" nimmt
		// mehrere Werte, und dahinter stuende der Text sonst als Werkzeugname.
		"--json-schema", schema,
		text.String(),
	}

	var (
		ergebnis schlusszeile
		gelesen  bool
	)
	laufFehler := laufen(ctx, ordner, func(zeile []byte) {
		var e schlusszeile
		if json.Unmarshal(zeile, &e) == nil && e.Typ == "result" {
			ergebnis, gelesen = e, true
		}
	}, argumente...)
	if laufFehler != nil {
		return antwort{}, "", laufFehler
	}
	if !gelesen {
		return antwort{}, "", errors.New("claude hat keine Antwort geschickt")
	}
	if ergebnis.Fehler || (ergebnis.Untertyp != "" && ergebnis.Untertyp != "success") {
		grund := strings.TrimSpace(ergebnis.Ergebnis)
		if grund == "" {
			grund = ergebnis.Untertyp
		}
		return antwort{}, "", fmt.Errorf("claude: %s", kuerzen(grund, 400))
	}
	if len(ergebnis.Struktur) == 0 || string(ergebnis.Struktur) == "null" {
		return antwort{}, "", errors.New("claude hat keine Schätzung in der verlangten Form geliefert")
	}
	var a antwort
	if err := json.Unmarshal(ergebnis.Struktur, &a); err != nil {
		return antwort{}, "", fmt.Errorf("Schätzung nicht lesbar: %w", err)
	}
	return a, ergebnis.modell(), nil
}

// schlusszeile ist die eine Zeile, die `--output-format json` ausgibt.
type schlusszeile struct {
	Typ      string          `json:"type"`
	Untertyp string          `json:"subtype"`
	Fehler   bool            `json:"is_error"`
	Ergebnis string          `json:"result"`
	Struktur json.RawMessage `json:"structured_output"`
	Modelle  map[string]struct {
		Aus int `json:"outputTokens"`
	} `json:"modelUsage"`
}

// modell nennt das Modell, das die Antwort geschrieben hat — das mit den
// meisten ausgegebenen Token. Die CLI ruft nebenher mitunter ein zweites.
func (e schlusszeile) modell() string {
	name, meiste := Modell, -1
	for m, v := range e.Modelle {
		if v.Aus > meiste {
			name, meiste = m, v.Aus
		}
	}
	return name
}

// angegeben sammelt, was am Handy schon eingetippt war.
func angegeben(n tagebuch.Naehrwerte) map[string]float64 {
	m := map[string]float64{}
	setze := func(name string, w float64) {
		if w > 0 {
			m[name] = w
		}
	}
	setze("kcal", n.Kcal)
	setze("fett", n.Fett)
	setze("gesaettigt", n.Gesaettigt)
	setze("kohlenhydrate", n.Kohlenhydrate)
	setze("zucker", n.Zucker)
	setze("ballaststoffe", n.Ballaststoffe)
	setze("eiweiss", n.Eiweiss)
	setze("salz", n.Salz)
	if len(m) == 0 {
		return nil
	}
	return m
}
