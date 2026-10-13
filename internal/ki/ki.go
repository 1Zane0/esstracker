// Package ki laesst dich den Tag besprechen, ohne das Fenster zu verlassen.
//
// Gerufen wird die angemeldete claude-CLI, nicht die API:
//
//	claude -p --session-id <uuid> "…"   die erste Frage, Kennung selbst gesetzt
//	claude -p --resume <uuid> "…"       jede weitere in derselben Unterhaltung
//
// Das ist derselbe Kniff wie im Wachturm nebenan, und er hat denselben Grund:
// die selbst gesetzte Kennung macht aus Einzelfragen ein Gespraech, und der
// Weg ueber die CLI laeuft auf das angemeldete Konto statt auf einen zweiten
// Schluessel, den irgendwer verwalten muesste.
//
// Vor jede Frage haengt das Programm den Stand des Tagebuchs — heute in voller
// Aufloesung, die Wochen davor als Zeilen (siehe lage.go). Das Modell muss
// also nicht erst danach suchen, und wer nach zehn Minuten nachfragt, bekommt
// die Zahlen von jetzt und nicht die vom Anfang der Unterhaltung. Fuer alles
// Aeltere liegt `tagebuch.json` im Arbeitsordner; lesen darf es dort.
//
// Gelesen wird der Ereignisstrom (`--output-format stream-json`), Zeile fuer
// Zeile. Das ist der Unterschied zwischen "es tut sich was, warte" und dem,
// was im Terminal steht: welche Datei gerade offen ist, wie der Satz waechst.
package ki

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/1Zane0/esstracker/internal/speicher"
	"github.com/1Zane0/esstracker/internal/tagebuch"
)

const datei = "gespraech.json"

// Hoechstzahl begrenzt, wie viele Nachrichten das Fenster aufhebt. Die Sitzung
// bei der CLI behaelt den vollen Verlauf; das hier ist nur, was gezeichnet wird.
const Hoechstzahl = 120

// HoechstSchritte deckelt den laufenden Mitschnitt, AufgehobeneSchritte das,
// was bei der fertigen Antwort liegen bleibt. Nachlesbar soll er sein, die
// Ablage aber nicht mit Werkzeugausgaben volllaufen.
const (
	HoechstSchritte     = 200
	AufgehobeneSchritte = 40
)

// Aufwand ist die Denkstufe. Sie haengt an der Sitzung, nicht am Modell, und
// muss deshalb bei jedem Aufruf mit — auch beim Fortsetzen.
const Aufwand = "max"

// Modell ist fest Haiku, fuer das Gespraech wie fuer die Schaetzungen. Das
// ist so gewollt: hier wird ein Tag eingeordnet und eine Heidelbeere
// geschaetzt, und dafuer reicht das kleine Modell. Wie der Aufwand muss es bei
// jedem Aufruf mit — sonst nimmt die CLI, was in ihren Einstellungen steht.
const Modell = "haiku"

// Frist ist grosszuegig, aber nicht endlos: was zehn Minuten laeuft, haengt,
// und ein haengender Prozess sperrt jede weitere Frage. Daneben steht die
// Notbremse von Hand.
const Frist = 10 * time.Minute

// errAbbruch unterscheidet den Griff zur Notbremse von der Zeitgrenze — beides
// beendet den Prozess, aber nur eins davon war Absicht.
var errAbbruch = errors.New("vom Benutzer abgebrochen")

// Schritt ist eine Zeile aus dem Ereignisstrom: ein Werkzeugaufruf, ein Stueck
// Fliesstext, ein Hinweis. Zusammen sind sie das, was im Terminal wegläuft.
type Schritt struct {
	Zeit    time.Time `json:"zeit"`
	Art     string    `json:"art"`    // ruf | text | hinweis | fehler
	Titel   string    `json:"titel"`  // "Read(tagebuch.json)"
	Detail  string    `json:"detail"` // "412 Zeilen"
	Offen   bool      `json:"offen"`  // laeuft noch
	kennung string    // tool_use_id, nur intern
}

// Lauf ist der Stand einer Frage, die gerade bearbeitet wird.
type Lauf struct {
	Beginn   time.Time `json:"beginn"`
	Frage    string    `json:"frage"`
	Modell   string    `json:"modell"`
	Schritte []Schritt `json:"schritte"`
	TokenAus int       `json:"tokenAus"`
}

// Nachricht ist eine Zeile im Verlauf.
//
// Frage ist, was der Mensch getippt hat — ohne den Standblock, der beim Senden
// davorgehaengt wird. Der gehoert in die Unterhaltung, aber nicht ins Fenster:
// dort stuende sonst bei jeder Frage eine Tabelle, die daneben ohnehin steht.
type Nachricht struct {
	Zeit     time.Time `json:"zeit"`
	Rolle    string    `json:"rolle"` // du | claude | trenner
	Text     string    `json:"text"`
	Fehler   bool      `json:"fehler,omitempty"`
	Dauer    string    `json:"dauer,omitempty"`
	Schritte []Schritt `json:"schritte,omitempty"`
}

// Gespraech ist der Verlauf.
type Gespraech struct {
	SitzungID   string      `json:"sitzungID"`
	Nachrichten []Nachricht `json:"nachrichten"`
	Begonnen    time.Time   `json:"begonnen"`
}

// Berater haelt das Gespraech und die eine Frage, die gerade laeuft.
//
// Es gibt genau ein Gespraech und hoechstens eine laufende Frage. Zwei Fragen
// an dieselbe Sitzung wuerden sich in der CLI ueberholen, und ein zweites
// Gespraech ueber dieselben Daten haette nichts, was das erste nicht hat.
type Berater struct {
	sperre  sync.Mutex
	buch    *tagebuch.Buch
	g       Gespraech
	lauf    *Lauf
	stoppen context.CancelCauseFunc
}

// Oeffne laedt den Verlauf von der Platte.
func Oeffne(b *tagebuch.Buch) (*Berater, error) {
	r := &Berater{buch: b}
	if err := speicher.Lies(datei, &r.g); err != nil {
		return nil, err
	}
	if r.g.Nachrichten == nil {
		r.g.Nachrichten = []Nachricht{}
	}
	return r, nil
}

// Bereit sagt, ob ueberhaupt gefragt werden kann — und wenn nicht, warum. Das
// gehoert ins Fenster, bevor jemand tippt, nicht in die Antwort danach.
func Bereit() (bool, string) {
	if _, err := claudePfad(); err != nil {
		return false, "Die claude-CLI ist nicht im PATH. Ohne sie gibt es hier niemanden zu fragen."
	}
	return true, ""
}

// Hole gibt den Verlauf heraus und, falls gerade etwas laeuft, dessen Stand.
// Beides als Kopie: sonst schriebe der Ereignisstrom weiter, waehrend das
// Fenster liest.
func (r *Berater) Hole() (Gespraech, *Lauf) {
	r.sperre.Lock()
	defer r.sperre.Unlock()

	kopie := r.g
	// Nie nil herausgeben: die Oberflaeche zaehlt die Nachrichten, und eine
	// fehlende Liste ist dort etwas anderes als eine leere.
	kopie.Nachrichten = append(make([]Nachricht, 0, len(r.g.Nachrichten)), r.g.Nachrichten...)
	return kopie, r.laufKopie()
}

// laufKopie gibt den Lauf heraus, ohne den Zeiger aus der Hand zu geben.
// Aufrufer haelt die Sperre.
func (r *Berater) laufKopie() *Lauf {
	if r.lauf == nil {
		return nil
	}
	k := *r.lauf
	k.Schritte = append([]Schritt(nil), r.lauf.Schritte...)
	return &k
}

// Neu wirft die Sitzungskennung weg; die naechste Frage beginnt von vorn. Der
// bisherige Verlauf bleibt stehen, mit einer Trennmarke — er ist schliesslich
// nicht falsch geworden.
func (r *Berater) Neu() error {
	r.sperre.Lock()
	defer r.sperre.Unlock()

	if r.lauf != nil {
		return errors.New("erst die laufende Frage abwarten oder abbrechen")
	}
	if r.g.SitzungID == "" && len(r.g.Nachrichten) == 0 {
		return nil
	}
	r.g.SitzungID = ""
	r.g.Nachrichten = append(r.g.Nachrichten, Nachricht{Zeit: time.Now(), Rolle: "trenner", Text: "Neue Unterhaltung"})
	r.stutzen()
	return r.sichern()
}

// Vergessen raeumt den ganzen Verlauf weg.
func (r *Berater) Vergessen() error {
	r.sperre.Lock()
	defer r.sperre.Unlock()

	if r.lauf != nil {
		return errors.New("erst die laufende Frage abwarten oder abbrechen")
	}
	r.g = Gespraech{Nachrichten: []Nachricht{}}
	return r.sichern()
}

// Abbrechen stoppt die laufende Frage — das Gegenstueck zu Esc im Terminal.
func (r *Berater) Abbrechen() error {
	r.sperre.Lock()
	stopp := r.stoppen
	r.sperre.Unlock()

	if stopp == nil {
		return errors.New("da läuft gerade nichts")
	}
	stopp(errAbbruch)
	return nil
}

func (r *Berater) sichern() error { return speicher.Schreib(datei, r.g) }

// stutzen haelt den angezeigten Verlauf kurz. Aufrufer haelt die Sperre.
func (r *Berater) stutzen() {
	if len(r.g.Nachrichten) > Hoechstzahl {
		r.g.Nachrichten = r.g.Nachrichten[len(r.g.Nachrichten)-Hoechstzahl:]
	}
}

// Fragen schickt eine Frage los und arbeitet sie im Hintergrund ab. Der Aufruf
// kehrt sofort zurueck; das Fenster fragt den Stand nach.
func (r *Berater) Fragen(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("leere Frage")
	}
	if len([]rune(text)) > 4000 {
		return errors.New("das ist zu lang für eine Frage")
	}
	if bereit, warum := Bereit(); !bereit {
		return errors.New(warum)
	}

	ordner := speicher.Ordner()
	if err := os.MkdirAll(ordner, 0o700); err != nil {
		return fmt.Errorf("datenordner anlegen: %w", err)
	}

	// Der Standblock wird hier gebaut, nicht erst im Hintergrund: so gilt
	// der Stand von dem Moment, in dem gefragt wurde.
	voll := lage(r.buch.Sicht()) + "\n\nFrage: " + text

	r.sperre.Lock()
	if r.lauf != nil {
		r.sperre.Unlock()
		return errors.New("es läuft schon eine Frage — erst die Antwort abwarten")
	}
	if r.g.Begonnen.IsZero() {
		r.g.Begonnen = time.Now()
	}
	r.g.Nachrichten = append(r.g.Nachrichten, Nachricht{Zeit: time.Now(), Rolle: "du", Text: text})
	r.stutzen()
	r.lauf = &Lauf{Beginn: time.Now(), Frage: text, Schritte: []Schritt{}}
	sitzung := r.g.SitzungID
	_ = r.sichern()
	r.sperre.Unlock()

	go r.arbeite(ordner, sitzung, voll)
	return nil
}

func (r *Berater) arbeite(ordner, sitzung, voll string) {
	beginn := time.Now()
	frist, endeFrist := context.WithTimeout(context.Background(), Frist)
	defer endeFrist()
	ctx, stopp := context.WithCancelCause(frist)
	defer stopp(nil)

	r.sperre.Lock()
	r.stoppen = stopp
	r.sperre.Unlock()

	melde := melder(func(aendern func(*Lauf)) {
		r.sperre.Lock()
		if r.lauf != nil {
			aendern(r.lauf)
			if len(r.lauf.Schritte) > HoechstSchritte {
				r.lauf.Schritte = r.lauf.Schritte[len(r.lauf.Schritte)-HoechstSchritte:]
			}
		}
		r.sperre.Unlock()
	})

	antwort, neueSitzung, err := frage(ctx, ordner, sitzung, voll, melde)

	n := Nachricht{Zeit: time.Now(), Rolle: "claude", Dauer: time.Since(beginn).Round(time.Second).String()}
	if err != nil {
		n.Fehler, n.Text = true, err.Error()
	} else {
		n.Text = antwort
	}

	r.sperre.Lock()
	if r.lauf != nil {
		n.Schritte = aufheben(r.lauf.Schritte)
	}
	r.g.Nachrichten = append(r.g.Nachrichten, n)
	if neueSitzung != "" {
		r.g.SitzungID = neueSitzung
	}
	r.stutzen()
	r.lauf = nil
	r.stoppen = nil
	_ = r.sichern()
	r.sperre.Unlock()
}

// aufheben behaelt vom Mitschnitt das, was spaeter noch etwas erzaehlt: die
// Werkzeugaufrufe. Der Fliesstext steht als Antwort daneben; ihn zweimal
// abzulegen waere nur Ballast.
func aufheben(schritte []Schritt) []Schritt {
	behalten := make([]Schritt, 0, len(schritte))
	for _, s := range schritte {
		if s.Art == "text" {
			continue
		}
		s.Offen = false
		if len([]rune(s.Detail)) > 300 {
			s.Detail = string([]rune(s.Detail)[:300]) + "…"
		}
		behalten = append(behalten, s)
	}
	if len(behalten) > AufgehobeneSchritte {
		behalten = behalten[len(behalten)-AufgehobeneSchritte:]
	}
	return behalten
}

// ---------- Mitschnitt ----------

// melder ist der Draht vom Ereignisstrom in den laufenden Auftrag.
type melder func(func(*Lauf))

func (m melder) schritt(s Schritt) {
	s.Zeit = time.Now()
	m(func(l *Lauf) { l.Schritte = append(l.Schritte, s) })
}

// text schreibt Fliesstext an den letzten offenen Textschritt weiter, statt
// fuer jedes Bruchstueck eine neue Zeile zu beginnen: so waechst der Satz im
// Fenster so, wie er im Terminal waechst.
func (m melder) text(stueck string) {
	m(func(l *Lauf) {
		if n := len(l.Schritte); n > 0 && l.Schritte[n-1].Art == "text" && l.Schritte[n-1].Offen {
			l.Schritte[n-1].Titel += stueck
			return
		}
		l.Schritte = append(l.Schritte, Schritt{Zeit: time.Now(), Art: "text", Titel: stueck, Offen: true})
	})
}

func (m melder) zumachen() {
	m(func(l *Lauf) {
		for i := range l.Schritte {
			if l.Schritte[i].Art == "text" {
				l.Schritte[i].Offen = false
			}
		}
	})
}

// erledigt hakt den Werkzeugaufruf ab, zu dem die Antwort gehoert.
func (m melder) erledigt(kennung, detail string, fehler bool) {
	m(func(l *Lauf) {
		for i := len(l.Schritte) - 1; i >= 0; i-- {
			if l.Schritte[i].kennung != kennung {
				continue
			}
			l.Schritte[i].Offen = false
			l.Schritte[i].Detail = detail
			if fehler {
				l.Schritte[i].Art = "fehler"
			}
			return
		}
	})
}

// ---------- Claude ----------

type block struct {
	Typ        string          `json:"type"`
	Text       string          `json:"text"`
	Name       string          `json:"name"`
	ID         string          `json:"id"`
	Eingabe    map[string]any  `json:"input"`
	WerkzeugID string          `json:"tool_use_id"`
	Inhalt     json.RawMessage `json:"content"`
	Fehler     bool            `json:"is_error"`
}

type ereignis struct {
	Typ       string `json:"type"`
	Untertyp  string `json:"subtype"`
	Modell    string `json:"model"`
	Ergebnis  string `json:"result"`
	Fehler    bool   `json:"is_error"`
	Nachricht *struct {
		Inhalt json.RawMessage `json:"content"`
	} `json:"message"`
	Strom *struct {
		Typ       string `json:"type"`
		Nachricht *struct {
			Modell string `json:"model"`
		} `json:"message"`
		Delta *struct {
			Text string `json:"text"`
		} `json:"delta"`
		Verbrauch *struct {
			Aus int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"event"`
}

func frage(ctx context.Context, ordner, sitzung, text string, m melder) (antwort, neueSitzung string, err error) {
	argumente := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--model", Modell,
		"--effort", Aufwand,
		"--append-system-prompt", anweisung,
		// Lesen genuegt: hier wird eingeordnet, nicht eingetragen. Ein
		// Eintrag ist ohnehin unveraenderlich, und `tagebuch.json` von
		// aussen umzuschreiben waere der eine Weg, das zu umgehen.
		//
		// Beide Listen muessen mit: die erlaubte genehmigt nur vorab,
		// gesperrt wird erst durch die zweite. Ohne sie greift die CLI
		// zur Schale, sobald ihr das schneller vorkommt — geprueft, sie
		// tut es. Netz steht auch darin: im Fenster steht der Satz, dass
		// nach draussen nur die Frage samt Stand geht, und WebFetch
		// waere die Tuer, durch die das nicht mehr stimmt.
		//
		// Eine Liste je Schalter, mit Komma: der Schalter frisst
		// mehrere Werte, und die Frage steht als letztes Argument
		// dahinter.
		"--allowedTools", "Read,Grep,Glob",
		"--disallowedTools", "Bash,Write,Edit,NotebookEdit,Task,WebFetch,WebSearch",
	}
	if sitzung == "" {
		// Die Kennung wird hier gewuerfelt statt von der CLI geholt: so
		// steht sie fest, bevor der erste Aufruf laeuft.
		neueSitzung = uuid()
		argumente = append(argumente, "--session-id", neueSitzung)
	} else {
		argumente = append(argumente, "--resume", sitzung)
	}
	argumente = append(argumente, text)

	var ergebnis, gesammelt strings.Builder
	var scheitern string
	// Welcher Aufruf zu welcher Antwort gehoert, steht nur in der Kennung.
	namen := map[string]string{}

	laufFehler := laufen(ctx, ordner, func(zeile []byte) {
		var e ereignis
		if json.Unmarshal(zeile, &e) != nil {
			return
		}
		switch e.Typ {
		case "system":
			if e.Untertyp == "init" && e.Modell != "" {
				modell := e.Modell
				m(func(l *Lauf) { l.Modell = modell })
			}

		case "stream_event":
			if e.Strom == nil {
				return
			}
			switch e.Strom.Typ {
			case "message_start":
				if e.Strom.Nachricht != nil && e.Strom.Nachricht.Modell != "" {
					modell := e.Strom.Nachricht.Modell
					m(func(l *Lauf) { l.Modell = modell })
				}
			case "content_block_delta":
				if d := e.Strom.Delta; d != nil && d.Text != "" {
					m.text(d.Text)
					gesammelt.WriteString(d.Text)
				}
			case "message_delta":
				// Erst hier stehen die wirklich verbrauchten Token;
				// die Zwischenstaende beim Schreiben sind Platzhalter.
				if v := e.Strom.Verbrauch; v != nil && v.Aus > 0 {
					raus := v.Aus
					m(func(l *Lauf) { l.TokenAus += raus })
				}
			case "message_stop":
				m.zumachen()
			}

		case "assistant":
			// Der fertige Block nennt den Werkzeugaufruf samt Eingabe;
			// die Bruchstuecke davor sind halbe JSON-Schnipsel.
			for _, blk := range bloecke(e) {
				if blk.Typ != "tool_use" {
					continue
				}
				m.zumachen()
				namen[blk.ID] = blk.Name
				m.schritt(Schritt{Art: "ruf", Offen: true, kennung: blk.ID, Titel: werkzeugZeile(blk.Name, blk.Eingabe)})
			}

		case "user":
			for _, blk := range bloecke(e) {
				if blk.Typ != "tool_result" {
					continue
				}
				m.erledigt(blk.WerkzeugID, ergebnisZeile(blk.Inhalt), blk.Fehler)
			}

		case "result":
			if e.Fehler || (e.Untertyp != "" && e.Untertyp != "success") {
				scheitern = strings.TrimSpace(e.Ergebnis)
				if scheitern == "" {
					scheitern = e.Untertyp
				}
				return
			}
			ergebnis.WriteString(e.Ergebnis)
		}
	}, argumente...)

	if laufFehler != nil {
		return "", "", laufFehler
	}
	if scheitern != "" {
		return "", "", fmt.Errorf("claude: %s", scheitern)
	}
	fertig := strings.TrimSpace(ergebnis.String())
	if fertig == "" {
		// Kein Ergebnisereignis (abgeschnittener Strom, uebergrosse
		// Zeile): dann eben das, was mitgelesen wurde.
		fertig = strings.TrimSpace(gesammelt.String())
	}
	if fertig == "" {
		return "", "", errors.New("claude hat keine Antwort geschickt")
	}
	return fertig, neueSitzung, nil
}

// bloecke holt die Inhaltsbloecke aus einem Ereignis. Sie sind nicht immer
// eine Liste — bei schlichtem Text steht dort eine Zeichenkette.
func bloecke(e ereignis) []block {
	if e.Nachricht == nil || len(e.Nachricht.Inhalt) == 0 {
		return nil
	}
	var liste []block
	if json.Unmarshal(e.Nachricht.Inhalt, &liste) != nil {
		return nil
	}
	return liste
}

// werkzeugZeile schreibt einen Aufruf so, wie ihn das Terminal zeigt: Name und
// das eine Feld, auf das es ankommt.
func werkzeugZeile(name string, eingabe map[string]any) string {
	feld := func(schluessel ...string) string {
		for _, s := range schluessel {
			if w, da := eingabe[s].(string); da && strings.TrimSpace(w) != "" {
				return strings.TrimSpace(w)
			}
		}
		return ""
	}
	wert := feld("file_path", "path", "pattern", "command", "query", "description")
	if wert == "" {
		return name
	}
	return name + "(" + kuerzen(einzeilig(wert), 70) + ")"
}

// ergebnisZeile fasst zusammen, was zurueckkam — eine Zeile, kein Dateiinhalt.
func ergebnisZeile(roh json.RawMessage) string {
	if len(roh) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(roh, &text) != nil {
		var liste []block
		if json.Unmarshal(roh, &liste) != nil {
			return ""
		}
		var teile []string
		for _, b := range liste {
			if b.Text != "" {
				teile = append(teile, b.Text)
			}
		}
		text = strings.Join(teile, " ")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if zeilen := strings.Count(text, "\n") + 1; zeilen > 1 {
		return fmt.Sprintf("%d Zeilen", zeilen)
	}
	return kuerzen(einzeilig(text), 90)
}

// laufen startet die CLI im Datenordner und liest ihre Ausgabe Zeile fuer
// Zeile mit, statt am Ende einen Klumpen einzusammeln: nur so kann das Fenster
// zeigen, was gerade passiert, statt nur dass etwas passiert.
func laufen(ctx context.Context, ordner string, aufZeile func([]byte), argumente ...string) error {
	pfad, err := claudePfad()
	if err != nil {
		return fmt.Errorf("claude liess sich nicht starten: %w", err)
	}
	befehl := exec.CommandContext(ctx, pfad, argumente...)
	befehl.Dir = ordner
	befehl.Stdin = nil
	ohneKonsole(befehl)

	rohr, err := befehl.StdoutPipe()
	if err != nil {
		return err
	}
	var fehl bytes.Buffer
	befehl.Stderr = &fehl
	if err := befehl.Start(); err != nil {
		return fmt.Errorf("claude liess sich nicht starten: %w", err)
	}

	leser := bufio.NewScanner(rohr)
	// Eine Ereigniszeile traegt auch mal eine ganze Datei mit sich.
	leser.Buffer(make([]byte, 0, 64<<10), 32<<20)
	for leser.Scan() {
		aufZeile(leser.Bytes())
	}
	// Bleibt nach einer uebergrossen Zeile etwas stehen, muss es trotzdem
	// abfliessen — sonst haengt die CLI an der vollen Leitung.
	_, _ = io.Copy(io.Discard, rohr)

	err = befehl.Wait()
	if err == nil {
		return nil
	}
	if errors.Is(context.Cause(ctx), errAbbruch) {
		return errors.New("abgebrochen")
	}
	if ctx.Err() != nil {
		return fmt.Errorf("claude hat nach %s nicht geantwortet — abgebrochen", Frist)
	}
	meldung := strings.TrimSpace(fehl.String())
	if meldung == "" {
		meldung = err.Error()
	}
	if len(meldung) > 1200 {
		meldung = meldung[:1200] + "…"
	}
	return fmt.Errorf("claude: %s", meldung)
}

func einzeilig(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func kuerzen(s string, hoechstens int) string {
	laeufer := []rune(s)
	if len(laeufer) <= hoechstens {
		return s
	}
	return string(laeufer[:hoechstens]) + "…"
}

// uuid wuerfelt eine Kennung in der Form, die `claude --session-id` erwartet.
func uuid() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Kann nicht scheitern, ohne dass Groesseres kaputt ist — und
		// eine Zeitkennung ist besser als gar keine Sitzung.
		return fmt.Sprintf("%08x-0000-4000-8000-%012x", time.Now().Unix(), time.Now().UnixNano()&0xffffffffffff)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
