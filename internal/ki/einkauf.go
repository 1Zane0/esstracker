package ki

// Der Planer schreibt die Einkaufsliste.
//
// Er sieht dasselbe wie der Reiter KI — den Stand des Tagebuchs, Tag fuer Tag,
// samt den haeufigsten Produkten — und dazu die Liste, die schon steht, und
// den Wunsch, den jemand ins Feld getippt hat. Zurueck kommen Posten in einem
// festen Schema, und eingetragen wird vom Programm.
//
// Wie beim Schaetzen: keine Werkzeuge, keine Sitzung, kein Mitschnitt. Ein
// Aufruf kann hier nichts anderes tun, als Posten vorzuschlagen — was davon
// auf die Liste kommt, entscheidet der Code nebenan.
//
// Preise kommen hier nicht vor. Dieses Programm kennt das Tagebuch und nicht
// die Regale; eine geratene Zahl mit Euro dahinter waere schlechter als keine.
// Es gibt Mengen, und die lassen sich im Fenster und am Handy mit + und −
// verstellen.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/1Zane0/esstracker/internal/speicher"
	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// PostenJeAufruf deckelt, wie viele Posten ein Aufruf bringen darf. Mehr ist
// kein Einkaufszettel mehr.
const PostenJeAufruf = 60

// Planer haelt den einen Auftrag, der gerade laeuft.
//
// Hoechstens einer: zwei Listen, die gleichzeitig entstehen, wuerden sich beim
// Eintragen ueberholen, und die zweite wuesste nichts von der ersten.
type Planer struct {
	buch *tagebuch.Buch

	sperre  sync.Mutex
	laeuft  bool
	seit    time.Time
	wunsch  string
	fehler  string
	zuletzt time.Time
	stoppen context.CancelCauseFunc
}

// NeuerPlaner baut den Planer. Er tut nichts, bis jemand eine Liste verlangt.
func NeuerPlaner(b *tagebuch.Buch) *Planer { return &Planer{buch: b} }

// Stand gibt heraus, was die Oberflaeche zeigt.
func (p *Planer) Stand() tagebuch.PlanStand {
	bereit, hinweis := Bereit()

	p.sperre.Lock()
	defer p.sperre.Unlock()

	st := tagebuch.PlanStand{
		Bereit: bereit, Hinweis: hinweis,
		Laeuft: p.laeuft, Wunsch: p.wunsch, Fehler: p.fehler,
	}
	if !p.seit.IsZero() {
		st.Seit = p.seit.Format(time.RFC3339)
	}
	if !p.zuletzt.IsZero() {
		st.Zuletzt = p.zuletzt.Format(time.RFC3339)
	}
	return st
}

// Erstellen laesst Claude eine Liste schreiben und kehrt sofort zurueck; das
// Fenster fragt den Stand nach.
//
// ergaenzen haengt die Vorschlaege an die bestehende Liste, sonst tritt die
// neue an ihre Stelle. Der Wunsch darf leer sein — dann entscheidet allein das
// Tagebuch, was fehlt.
func (p *Planer) Erstellen(wunsch string, ergaenzen bool) error {
	wunsch = strings.TrimSpace(wunsch)
	if len([]rune(wunsch)) > 1000 {
		return errors.New("das ist zu lang für einen Wunsch")
	}
	if bereit, warum := Bereit(); !bereit {
		return errors.New(warum)
	}

	ordner := speicher.Ordner()
	if err := os.MkdirAll(ordner, 0o700); err != nil {
		return fmt.Errorf("datenordner anlegen: %w", err)
	}

	// Stand und Liste werden hier gelesen, nicht erst im Hintergrund: es gilt,
	// was in dem Moment dastand, in dem jemand gedrueckt hat.
	text := auftrag(lage(p.buch.Sicht()), p.buch.EinkaufHolen(), wunsch, ergaenzen)

	p.sperre.Lock()
	if p.laeuft {
		p.sperre.Unlock()
		return errors.New("es wird schon an einer Liste geschrieben — erst die abwarten")
	}
	p.laeuft, p.seit, p.wunsch, p.fehler = true, time.Now(), wunsch, ""
	p.sperre.Unlock()

	go p.arbeite(ordner, text, wunsch, ergaenzen)
	return nil
}

// Abbrechen stoppt den laufenden Aufruf. Die Liste, die dasteht, bleibt.
func (p *Planer) Abbrechen() error {
	p.sperre.Lock()
	stopp := p.stoppen
	p.sperre.Unlock()

	if stopp == nil {
		return errors.New("da wird gerade keine Liste geschrieben")
	}
	stopp(errAbbruch)
	return nil
}

func (p *Planer) arbeite(ordner, text, wunsch string, ergaenzen bool) {
	frist, endeFrist := context.WithTimeout(context.Background(), Frist)
	defer endeFrist()
	ctx, stopp := context.WithCancelCause(frist)
	defer stopp(nil)

	p.sperre.Lock()
	p.stoppen = stopp
	p.sperre.Unlock()

	fehler := ""
	vorschlag, modell, err := plane(ctx, ordner, text)
	switch {
	case err != nil:
		fehler = err.Error()
	default:
		posten := vorschlag.posten()
		if len(posten) == 0 {
			fehler = "Claude hat keinen brauchbaren Posten geliefert."
		} else if err := p.buch.EinkaufSchreiben(posten, !ergaenzen, wunsch, vorschlag.Notiz, modell); err != nil {
			fehler = err.Error()
		}
	}

	p.sperre.Lock()
	p.laeuft, p.seit, p.stoppen = false, time.Time{}, nil
	p.fehler, p.zuletzt = fehler, time.Now()
	p.sperre.Unlock()
}

// ---------- Claude ----------

// einkaufAnweisung ist der ganze Systemtext — an der Stelle des Systemtexts
// von Claude Code, nicht dahinter. Dort steht, wie man programmiert; hier geht
// es um einen Einkaufszettel.
const einkaufAnweisung = `Du schreibst die Einkaufsliste für ein Essenstagebuch. Vor dir steht der Stand
des Tagebuchs — was zuletzt gegessen wurde, wie viel davon, und das Tagesziel —,
die Liste, die bereits steht, und der Wunsch des Menschen.

Deine Posten gehen ohne Rückfrage auf die Liste. Sie landen am Handy und werden
damit eingekauft.

Für jeden Posten:

- name: das Lebensmittel, wie es im Laden heißt und wie man es auf einen Zettel
  schreibt — "Vollkornbrot", "Magerquark", "Bananen". Keine Sätze, keine
  Mengenangabe im Namen, keine Marke, außer sie steht so im Tagebuch.
- menge und einheit: eine Zahl und eine der vorgegebenen Einheiten. Nimm
  Haushaltsmengen, so wie man sie kauft: 500 g Hackfleisch, 1 kg Kartoffeln,
  6 Stück Eier, 1 Packung Butter. Die Zahl lässt sich später mit + und −
  verstellen; setz sie auf das, was üblich ist.
- gruppe: die Abteilung aus der Vorgabe, damit die Liste im Laden in einem Gang
  abzuarbeiten ist. Passt nichts, nimm "Sonstiges".
- grund: ein knapper Halbsatz, warum der Posten draufsteht — "4× diese Woche
  gegessen", "für den Eiweißanteil", "war im Wunsch". Keine Ratschläge, keine
  Bewertung dessen, was jemand isst.

Preise nennst du nicht — nirgends, auch nicht im Grund oder in der Notiz. Du
kennst sie nicht, und eine geschätzte Zahl sieht hier aus wie eine bekannte.

notiz: ein oder zwei Sätze zur Liste als Ganzes. Woran du dich gehalten hast,
was du weggelassen hast, was auffiel. Kein Vorwort, keine Wiederholung der
Posten.

Richte dich nach dem Wunsch, wenn einer dasteht — Tage, Anzahl der Mahlzeiten,
Vorlieben, was nicht ins Haus soll. Steht keiner, schreib eine Liste, die zu
dem passt, was im Tagebuch ohnehin regelmäßig vorkommt, und ergänze, was
dabei offensichtlich fehlt.

Sind Posten schon auf der Liste, schlag sie nicht erneut vor — auch nicht in
anderer Schreibweise.

Halt die Liste in einer Länge, die man in einem Einkauf abarbeitet: ohne
anderen Wunsch zwischen zehn und fünfundzwanzig Posten.

Du bist kein Arzt und keine Ernährungsberatung. Mengen und Lebensmittel: ja.
Diagnosen, Therapien, Nährstoffversprechen, Warnungen vor Krankheiten: nein.`

// einkaufSchema legt die Antwort fest. Die Einheiten und Abteilungen kommen
// aus dem Tagebuch: das Schema kann so nicht neben der Liste herlaufen, die
// das Programm kennt, und die CLI prueft es, bevor etwas zurueckkommt.
var einkaufSchema = func() string {
	einheiten := make([]string, 0, len(tagebuch.Einheiten))
	for _, e := range tagebuch.Einheiten {
		einheiten = append(einheiten, e.Eins)
	}
	alsListe := func(werte []string) string {
		roh, _ := json.Marshal(werte)
		return string(roh)
	}
	return `{
  "type": "object",
  "properties": {
    "posten": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "menge": {"type": "number"},
          "einheit": {"type": "string", "enum": ` + alsListe(einheiten) + `},
          "gruppe": {"type": "string", "enum": ` + alsListe(tagebuch.Gruppen) + `},
          "grund": {"type": "string"}
        },
        "required": ["name", "menge", "einheit", "gruppe", "grund"],
        "additionalProperties": false
      }
    },
    "notiz": {"type": "string"}
  },
  "required": ["posten", "notiz"],
  "additionalProperties": false
}`
}()

// listenAntwort ist, was Claude schickt.
type listenAntwort struct {
	Posten []struct {
		Name    string  `json:"name"`
		Menge   float64 `json:"menge"`
		Einheit string  `json:"einheit"`
		Gruppe  string  `json:"gruppe"`
		Grund   string  `json:"grund"`
	} `json:"posten"`
	Notiz string `json:"notiz"`
}

// posten macht aus der Antwort Zeilen fuers Tagebuch. Geprueft wird dort — hier
// wird nur umgeschrieben und gedeckelt.
func (a listenAntwort) posten() []tagebuch.Posten {
	liste := make([]tagebuch.Posten, 0, len(a.Posten))
	for _, v := range a.Posten {
		if len(liste) == PostenJeAufruf {
			break
		}
		liste = append(liste, tagebuch.Posten{
			Name:    v.Name,
			Menge:   v.Menge,
			Einheit: v.Einheit,
			Gruppe:  v.Gruppe,
			Grund:   v.Grund,
			Von:     tagebuch.VonKI,
		})
	}
	return liste
}

// auftrag schreibt zusammen, was Claude zu sehen bekommt: der Stand des
// Tagebuchs, die Liste von jetzt, der Wunsch.
func auftrag(stand string, e tagebuch.Einkauf, wunsch string, ergaenzen bool) string {
	var b strings.Builder
	b.WriteString(stand)
	b.WriteString("\n\n<liste>\n")
	if len(e.Posten) == 0 {
		b.WriteString("Die Einkaufsliste ist leer.\n")
	} else {
		for _, p := range e.Posten {
			zustand := "offen"
			if p.Erledigt {
				zustand = "schon eingekauft"
			}
			fmt.Fprintf(&b, "- %s · %s · %s · %s\n", feld(p.Name), p.MengeText(), p.Gruppe, zustand)
		}
	}
	b.WriteString("</liste>\n\n")

	if ergaenzen {
		b.WriteString("<auftrag art=\"ergänzen\">\nDie Liste oben bleibt stehen. Schlag nur Posten vor, die noch fehlen.\n")
	} else {
		b.WriteString("<auftrag art=\"neu\">\nDie Liste oben wird durch deine ersetzt.\n")
	}
	if wunsch == "" {
		b.WriteString("Es steht kein besonderer Wunsch dabei — richte dich nach dem Tagebuch.\n")
	} else {
		fmt.Fprintf(&b, "Wunsch: %s\n", wunsch)
	}
	b.WriteString("</auftrag>")
	return b.String()
}

func plane(ctx context.Context, ordner, text string) (listenAntwort, string, error) {
	argumente := []string{
		"-p", "--output-format", "json",
		"--model", Modell,
		"--effort", Aufwand,
		"--system-prompt", einkaufAnweisung,
		"--no-session-persistence",
		"--strict-mcp-config",
		"--tools", "",
		// Das Schema als letzter Schalter vor dem Text: "--tools" nimmt
		// mehrere Werte, und dahinter stuende der Text sonst als Werkzeugname.
		"--json-schema", einkaufSchema,
		text,
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
		return listenAntwort{}, "", laufFehler
	}
	if !gelesen {
		return listenAntwort{}, "", errors.New("claude hat keine Antwort geschickt")
	}
	if ergebnis.Fehler || (ergebnis.Untertyp != "" && ergebnis.Untertyp != "success") {
		grund := strings.TrimSpace(ergebnis.Ergebnis)
		if grund == "" {
			grund = ergebnis.Untertyp
		}
		return listenAntwort{}, "", fmt.Errorf("claude: %s", kuerzen(grund, 400))
	}
	if len(ergebnis.Struktur) == 0 || string(ergebnis.Struktur) == "null" {
		return listenAntwort{}, "", errors.New("claude hat keine Liste in der verlangten Form geliefert")
	}
	var a listenAntwort
	if err := json.Unmarshal(ergebnis.Struktur, &a); err != nil {
		return listenAntwort{}, "", fmt.Errorf("Liste nicht lesbar: %w", err)
	}
	return a, ergebnis.modell(), nil
}
