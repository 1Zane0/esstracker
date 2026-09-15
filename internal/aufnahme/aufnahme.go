// Package aufnahme ist die Tuer, durch die das Handy hereinkommt.
//
// Sie ist absichtlich die einzige: die Oberflaeche haengt an der
// Loopback-Adresse und ist vom Netz aus nicht erreichbar. Hier draussen im
// Heimnetz steht nur, was ein Handy wirklich braucht — sich melden, sich
// koppeln, einen Eintrag abgeben.
//
// Die Kopplung laeuft ohne Tipparbeit und trotzdem nicht offen:
//
//   - Der Rechner antwortet auf einen UDP-Ruf ins Heimnetz mit seiner Adresse.
//     Das Handy muss die also nicht kennen und nicht erfragen.
//   - Ein Token bekommt aber nur, wer sich meldet, waehrend am Rechner ein
//     Kopplungsfenster offen steht. Das oeffnet der Mensch am Rechner per
//     Knopf, es schliesst sich nach fuenf Minuten von selbst und nimmt genau
//     ein Geraet auf.
//
// Damit ist der gefaehrliche Zustand nicht der Normalzustand, sondern eine
// Ausnahme, die jemand absichtlich herbeifuehrt und sieht. Wer waehrend
// dieser fuenf Minuten im selben Heimnetz mitliest, kann sich dazwischen
// draengen — das ist der Preis, und er ist hier benannt.
package aufnahme

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// Port ist die Tuer ins Heimnetz — TCP fuer die Eintraege, UDP fuer die Suche.
// 8790 liegt hinter der Oberflaeche auf 8789, die Tasklist auf 8788 und dem
// Wachturm auf 8787.
const Port = 8790

// KopplungDauer ist, wie lange das Fenster offen steht.
const KopplungDauer = 5 * time.Minute

// Ruf ist das Wort, auf das der UDP-Lauscher antwortet. Kurz und eindeutig,
// damit ein fremdes Programm im Netz nicht versehentlich eine Antwort ausloest.
const Ruf = "esstracker-wo-bist-du"

// Dienst nimmt Eintraege entgegen.
type Dienst struct {
	Buch *tagebuch.Buch

	// port ist der tatsaechlich genutzte Port, nicht die Vorgabe. Die
	// Oberflaeche zeigt ihn an, damit die Adresse dort auch dann stimmt, wenn
	// ESSTRACKER_EMPFANG etwas anderes sagt.
	port int

	sperre sync.Mutex
	bis    time.Time // bis wann das Kopplungsfenster offen ist
	wort   string    // sichtbares Wort, damit man am Handy erkennt, welcher Rechner antwortet
}

// Neu baut den Dienst fuer den Port, auf dem er wirklich lauscht.
func Neu(b *tagebuch.Buch, port int) *Dienst {
	if port <= 0 {
		port = Port
	}
	return &Dienst{Buch: b, port: port}
}

// PortAus liest den Port aus einer Adresse wie "0.0.0.0:8790". Was sich nicht
// lesen laesst, faellt auf die Vorgabe zurueck — ein Tippfehler in der
// Umgebungsvariable soll die Anzeige nicht auf null setzen.
func PortAus(adresse string) int {
	_, teil, err := net.SplitHostPort(adresse)
	if err != nil {
		return Port
	}
	n, err := strconv.Atoi(teil)
	if err != nil || n <= 0 {
		return Port
	}
	return n
}

// ---------- Kopplungsfenster ----------

// Oeffnen startet das Kopplungsfenster und gibt das Wort zurueck, das dabei
// auf beiden Seiten steht.
func (d *Dienst) Oeffnen() string {
	d.sperre.Lock()
	defer d.sperre.Unlock()

	d.bis = time.Now().Add(KopplungDauer)
	d.wort = wort()
	return d.wort
}

// Schliessen beendet das Fenster von Hand.
func (d *Dienst) Schliessen() {
	d.sperre.Lock()
	defer d.sperre.Unlock()
	d.bis = time.Time{}
	d.wort = ""
}

// Stand meldet, ob das Fenster offen ist und wie lange noch.
func (d *Dienst) Stand() tagebuch.Kopplung {
	d.sperre.Lock()
	offen := time.Now().Before(d.bis)
	rest := 0
	w := ""
	if offen {
		rest = int(time.Until(d.bis).Seconds())
		w = d.wort
	}
	d.sperre.Unlock()

	return tagebuch.Kopplung{
		Offen: offen, BisSek: rest, Wort: w,
		Adressen: Adressen(), Port: d.port,
	}
}

// wort baut ein gut vorlesbares Erkennungswort aus zwei Silbenlisten. Es
// schuetzt nichts — es sagt nur, welcher Rechner da antwortet, falls zwei im
// Netz stehen.
func wort() string {
	erste := []string{"Apfel", "Birne", "Hafer", "Kürbis", "Linse", "Mohn", "Nuss", "Olive", "Pilz", "Quitte", "Rübe", "Salbei"}
	zweite := []string{"Brot", "Teller", "Löffel", "Korb", "Topf", "Kelle", "Glas", "Schale"}
	return erste[zufallszahl(len(erste))] + zweite[zufallszahl(len(zweite))]
}

func zufallszahl(n int) int {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return 0
	}
	return (int(b[0])<<8 | int(b[1])) % n
}

func token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// Ohne Zufall kein Token: lieber scheitern als eines vergeben, das
		// jemand erraten kann.
		return ""
	}
	return hex.EncodeToString(b)
}

// ---------- HTTP ----------

// Handler verdrahtet die drei Pfade, die es hier draussen gibt.
func (d *Dienst) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hallo", d.hallo)
	mux.HandleFunc("POST /koppeln", d.koppeln)
	mux.HandleFunc("POST /aufnahme", d.eintrag)
	return kopfzeilen(mux)
}

// kopfzeilen setzt, was auf jede Antwort gehoert.
func kopfzeilen(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// hallo ist die Frage „bist du ein Esstracker und nimmst du gerade jemanden
// auf?". Sie braucht kein Token — sie verraet nichts, was nicht ohnehin an
// jedem offenen Port abzulesen waere.
func (d *Dienst) hallo(w http.ResponseWriter, r *http.Request) {
	k := d.Stand()
	json.NewEncoder(w).Encode(map[string]any{
		"dienst":   "esstracker",
		"rechner":  rechnername(),
		"kopplung": k.Offen,
		"wort":     k.Wort,
	})
}

type kopplungsWunsch struct {
	Geraet string `json:"geraet"` // stabile ID des Handys
	Name   string `json:"name"`   // wie es sich nennt
}

// koppeln gibt ein Token aus — aber nur, solange das Fenster offen ist.
func (d *Dienst) koppeln(w http.ResponseWriter, r *http.Request) {
	var wu kopplungsWunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	wu.Geraet = strings.TrimSpace(wu.Geraet)
	wu.Name = strings.TrimSpace(wu.Name)
	if wu.Geraet == "" {
		fehler(w, http.StatusBadRequest, errors.New("Kopplung ohne Geräte-ID"))
		return
	}
	if wu.Name == "" {
		wu.Name = "Handy"
	}

	d.sperre.Lock()
	offen := time.Now().Before(d.bis)
	d.sperre.Unlock()

	if !offen {
		fehler(w, http.StatusForbidden, errors.New("am Rechner ist gerade kein Kopplungsfenster offen"))
		return
	}

	t := token()
	if t == "" {
		fehler(w, http.StatusInternalServerError, errors.New("kein Zufall verfügbar"))
		return
	}
	g := tagebuch.Geraet{
		ID: wu.Geraet, Name: wu.Name, Token: t,
		Gekoppelt: time.Now().Format(time.RFC3339),
	}
	if err := d.Buch.GeraetMerken(g); err != nil {
		fehler(w, http.StatusInternalServerError, err)
		return
	}

	// Ein Fenster, ein Geraet: wer gekoppelt ist, schliesst die Tuer hinter
	// sich. Sonst steht sie fuenf Minuten fuer jeden offen, der es
	// gleichzeitig versucht.
	d.Schliessen()

	json.NewEncoder(w).Encode(map[string]any{
		"token":   t,
		"rechner": rechnername(),
		"ziel":    d.Buch.Tagesziel(),
	})
}

// eintrag nimmt ein gegessenes Produkt entgegen.
//
// Die Antwort sagt immer, ob der Eintrag neu war. Ein Nachzuegler, den der
// Rechner schon kennt, ist kein Fehler — das Handy soll ihn dann nur aus
// seiner Warteschlange nehmen.
func (d *Dienst) eintrag(w http.ResponseWriter, r *http.Request) {
	g, ok := d.Buch.GeraetZuToken(bearer(r))
	if !ok {
		fehler(w, http.StatusUnauthorized, errors.New("unbekanntes Token — bitte neu koppeln"))
		return
	}

	// Ein Paket oder ein Stapel: das Handy schickt beim Aufholen lieber
	// einmal zwanzig Eintraege als zwanzigmal einen.
	var stapel []tagebuch.Eintrag
	roh, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fehler(w, http.StatusBadRequest, errors.New("Anfrage konnte nicht gelesen werden"))
		return
	}
	if err := json.Unmarshal(roh, &stapel); err != nil {
		var einer tagebuch.Eintrag
		if err2 := json.Unmarshal(roh, &einer); err2 != nil {
			fehler(w, http.StatusBadRequest, errors.New("Anfrage war weder ein Eintrag noch eine Liste"))
			return
		}
		stapel = []tagebuch.Eintrag{einer}
	}
	if len(stapel) == 0 {
		fehler(w, http.StatusBadRequest, errors.New("leere Liste"))
		return
	}
	if len(stapel) > 500 {
		fehler(w, http.StatusBadRequest, errors.New("höchstens 500 Einträge auf einmal"))
		return
	}

	// Jeden einzeln bewerten und die IDs zurueckmelden, die liegen. Ein
	// fauler Eintrag im Stapel darf die anderen neunzehn nicht aufhalten —
	// sonst haengt die Warteschlange des Handys fuer immer.
	angenommen := make([]string, 0, len(stapel))
	abgelehnt := map[string]string{}
	neu := 0
	for _, e := range stapel {
		war, err := d.Buch.Aufnehmen(e, g.ID)
		switch {
		case err != nil:
			id := e.ID
			if id == "" {
				id = "(ohne ID)"
			}
			abgelehnt[id] = err.Error()
		case war:
			neu++
			angenommen = append(angenommen, e.ID)
		default:
			angenommen = append(angenommen, e.ID)
		}
	}

	json.NewEncoder(w).Encode(map[string]any{
		"angenommen": angenommen,
		"abgelehnt":  abgelehnt,
		"neu":        neu,
		"ziel":       d.Buch.Tagesziel(),
	})
}

// bearer zieht das Token aus der Kopfzeile.
func bearer(r *http.Request) string {
	k := r.Header.Get("Authorization")
	if len(k) > 7 && strings.EqualFold(k[:7], "Bearer ") {
		return strings.TrimSpace(k[7:])
	}
	return ""
}

func lies(r *http.Request, ziel any) error {
	roh, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		return errors.New("Anfrage konnte nicht gelesen werden")
	}
	if len(roh) == 0 {
		return nil
	}
	if err := json.Unmarshal(roh, ziel); err != nil {
		return errors.New("Anfrage war kein gültiges JSON")
	}
	return nil
}

func fehler(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"fehler": err.Error()})
}

// ---------- UDP-Suche ----------

// Lausche beantwortet den Ruf des Handys ins Heimnetz mit der eigenen Adresse.
//
// Das ist der Grund, warum am Handy keine IP eingetippt werden muss — und es
// laeuft bewusst nur ueber UDP-Rundruf im eigenen Netz, nicht ueber mDNS: ein
// Rundruf und eine Antwort sind vierzig Zeilen, mDNS waere eine Bibliothek.
func (d *Dienst) Lausche(fertig <-chan struct{}) error {
	verb, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", d.port))
	if err != nil {
		return fmt.Errorf("UDP-Suche auf %d: %w", d.port, err)
	}
	go func() {
		<-fertig
		verb.Close()
	}()

	puffer := make([]byte, 256)
	for {
		n, von, err := verb.ReadFrom(puffer)
		if err != nil {
			return nil // geschlossen
		}
		if strings.TrimSpace(string(puffer[:n])) != Ruf {
			continue
		}
		k := d.Stand()
		antwort, _ := json.Marshal(map[string]any{
			"dienst":   "esstracker",
			"rechner":  rechnername(),
			"port":     d.port,
			"kopplung": k.Offen,
			"wort":     k.Wort,
		})
		_, _ = verb.WriteTo(antwort, von)
	}
}

// ---------- Adressen ----------

// Adressen sind die IPv4-Adressen dieses Rechners im Heimnetz — das, was am
// Handy stehen muesste, wenn die Suche einmal nicht durchkommt.
func Adressen() []string {
	var liste []string
	schnitt, err := net.Interfaces()
	if err != nil {
		return []string{}
	}
	for _, s := range schnitt {
		if s.Flags&net.FlagUp == 0 || s.Flags&net.FlagLoopback != 0 {
			continue
		}
		adressen, err := s.Addrs()
		if err != nil {
			continue
		}
		for _, a := range adressen {
			netz, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := netz.IP.To4()
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			liste = append(liste, ip.String())
		}
	}
	if liste == nil {
		liste = []string{}
	}
	return liste
}

// rechnername ist das, was am Handy steht, damit man zwei Rechner im selben
// Netz auseinanderhaelt.
func rechnername() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "Rechner"
	}
	return h
}
