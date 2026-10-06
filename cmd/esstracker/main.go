// Esstracker — was gegessen wurde, vom Handy auf diesen Rechner.
//
// Das Handy erfasst und zaehlt Kalorien, dieser Rechner haelt alles: jedes
// Produkt, jede Uhrzeit, jeden Naehrwert. Zwei Tueren, mit Absicht getrennt:
//
//	127.0.0.1:8789   die Oberflaeche — nur fuer diesen Rechner
//	0.0.0.0:8790     der Empfang — nur fuer das gekoppelte Handy im Heimnetz
//
// Unterbefehle:
//
//	esstracker             Fenster oeffnen (Vorgabe)
//	esstracker dienst      nur ausliefern, ohne Fenster
//	esstracker heute       den heutigen Stand im Terminal zeigen
//	esstracker koppeln     Kopplungsfenster im Terminal oeffnen
//	esstracker ausfuhr     alles als CSV auf die Standardausgabe
//	esstracker einrichten  Starter im Anwendungsmenue anlegen
//	esstracker version     Fassung anzeigen
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/1Zane0/esstracker/internal/aufnahme"
	"github.com/1Zane0/esstracker/internal/fenster"
	"github.com/1Zane0/esstracker/internal/ki"
	"github.com/1Zane0/esstracker/internal/tagebuch"
	"github.com/1Zane0/esstracker/internal/web"
)

const fassung = "1.0.0"

// adresse ist die Loopback-Adresse der Oberflaeche. 8789 liegt direkt neben
// der Tasklist auf 8788 und dem Wachturm auf 8787 — alle drei sollen
// gleichzeitig offen sein duerfen.
func adresse() string {
	if a := os.Getenv("ESSTRACKER_ADRESSE"); a != "" {
		return a
	}
	return "127.0.0.1:8789"
}

// empfangsadresse ist die Tuer ins Heimnetz. Sie haengt bewusst nicht an der
// Loopback-Adresse — sonst kaeme das Handy nicht herein.
func empfangsadresse() string {
	if a := os.Getenv("ESSTRACKER_EMPFANG"); a != "" {
		return a
	}
	return fmt.Sprintf("0.0.0.0:%d", aufnahme.Port)
}

func main() {
	flag.Usage = hilfe
	flag.Parse()

	befehl := "fenster"
	if flag.NArg() > 0 {
		befehl = flag.Arg(0)
	}

	var err error
	switch befehl {
	case "fenster", "":
		err = programm()
	case "dienst", "serve":
		err = dienst()
	case "heute":
		err = heute()
	case "koppeln":
		err = koppeln()
	case "ausfuhr":
		err = ausfuhr()
	case "einrichten":
		err = einrichten()
	case "version":
		fmt.Println("esstracker " + fassung)
	case "hilfe", "-h", "--help", "help":
		hilfe()
	default:
		fmt.Fprintf(os.Stderr, "Unbekannter Befehl %q.\n\n", befehl)
		hilfe()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Fehler: "+err.Error())
		os.Exit(1)
	}
}

func hilfe() {
	fmt.Fprint(os.Stderr, `Esstracker `+fassung+` — was gegessen wurde, vom Handy auf diesen Rechner

  esstracker             Fenster öffnen (Vorgabe)
  esstracker dienst      nur ausliefern, ohne Fenster
  esstracker heute       den heutigen Stand im Terminal zeigen
  esstracker koppeln     Kopplungsfenster öffnen und warten
  esstracker ausfuhr     alles als CSV auf die Standardausgabe
  esstracker einrichten  Starter im Anwendungsmenü anlegen
  esstracker version     Fassung anzeigen

Umgebung:
  ESSTRACKER_DATEN     Datenverzeichnis (Vorgabe ~/.local/share/esstracker)
  ESSTRACKER_ADRESSE   Oberfläche (Vorgabe 127.0.0.1:8789)
  ESSTRACKER_EMPFANG   Empfang vom Handy (Vorgabe 0.0.0.0:8790)
`)
}

// ---------- Aufbau ----------

// teile baut Buch, Empfang und Berater und bringt beide Tueren hoch.
//
// Der Empfang darf scheitern, ohne das Programm mitzunehmen: ein belegter
// Port 8790 heisst, dass kein Handy durchkommt — aber die Daten, die schon da
// sind, soll man trotzdem ansehen koennen.
func teile() (*tagebuch.Buch, *aufnahme.Dienst, *ki.Berater, *ki.Schaetzer, *ki.Planer, error) {
	buch, err := tagebuch.Oeffne()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	// Der Berater laedt nur seinen Verlauf; ob die claude-CLI ueberhaupt da
	// ist, entscheidet sich erst bei der ersten Frage. Ein fehlendes
	// Werkzeug soll das Tagebuch nicht am Starten hindern.
	berater, err := ki.Oeffne(buch)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return buch, aufnahme.Neu(buch, aufnahme.PortAus(empfangsadresse())), berater,
		ki.NeuerSchaetzer(buch), ki.NeuerPlaner(buch), nil
}

// schaetzenAn verdrahtet das Nachschaetzen: was vom Handy ohne Naehrwerte
// kommt, schaetzt Claude von allein — und was schon wartet, weil der Rechner
// aus war, gleich beim Start. Nur dort, wo das Programm dauerhaft laeuft:
// `esstracker koppeln` soll nicht nebenbei Claude rufen.
func schaetzenAn(e *aufnahme.Dienst, s *ki.Schaetzer) {
	e.Offen = func() { s.Anstossen(false) }
	s.Anstossen(false)
}

// empfangHoch oeffnet die Tuer ins Heimnetz — TCP fuer die Eintraege, UDP fuer
// die Suche. Gibt zurueck, was beim Beenden zu schliessen ist.
func empfangHoch(e *aufnahme.Dienst, meckern bool) (zu func()) {
	fertig := make(chan struct{})
	adr := empfangsadresse()

	server := &http.Server{
		Addr:              adr,
		Handler:           e.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) && meckern {
			fmt.Fprintln(os.Stderr, "Empfang auf "+adr+": "+err.Error()+
				"\nDie Oberfläche läuft weiter, aber das Handy kommt nicht durch.")
		}
	}()
	go func() {
		if err := e.Lausche(fertig); err != nil && meckern {
			fmt.Fprintln(os.Stderr, "Suche: "+err.Error()+
				"\nDas Handy findet den Rechner dann nicht von allein — die Adresse steht in der Oberfläche.")
		}
	}()

	return func() {
		close(fertig)
		aus, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = server.Shutdown(aus)
	}
}

// ---------- Fenster ----------

// programm oeffnet das Fenster.
//
// Laeuft schon eine Fassung, zeigt das Fenster auf die: zwei Prozesse, die
// dieselbe Datei im Speicher halten und nacheinander schreiben, wuerden sich
// gegenseitig Eintraege ueberbuegeln. Ein Fenster mehr ist harmlos, ein
// zweiter Schreiber nicht.
func programm() error {
	adr := adresse()

	if erreichbar(adr) {
		return zeige(fenster.Oeffne("http://"+adr+"/", "Esstracker"))
	}

	buch, empfang, berater, schaetzer, planer, err := teile()
	if err != nil {
		return zeige(err)
	}
	lauscher, err := net.Listen("tcp", adr)
	if err != nil {
		return zeige(fmt.Errorf("%s ist belegt: %w", adr, err))
	}
	server := &http.Server{
		Handler:           web.Neu(buch, empfang, berater, schaetzer, planer).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	schaetzenAn(empfang, schaetzer)
	go func() {
		if err := server.Serve(lauscher); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "Server: "+err.Error())
		}
	}()
	empfangZu := empfangHoch(empfang, false)

	fehler := fenster.Oeffne("http://"+adr+"/", "Esstracker")

	// Das Fenster ist zu — die Server duerfen nicht weiterlaufen, sonst haelt
	// ein unsichtbarer Prozess die Datei und die Ports fest.
	empfangZu()
	aus, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_ = server.Shutdown(aus)
	return zeige(fehler)
}

// zeige bringt ein Scheitern dorthin, wo es jemand sieht.
//
// Aus dmenu gestartet gibt es kein Terminal: eine Meldung auf stderr faellt
// ins Leere, und das Programm sieht aus, als tue es einfach nichts. Dann ist
// eine Schreibtisch-Benachrichtigung der einzige Weg nach draussen.
func zeige(err error) error {
	if err == nil {
		return nil
	}
	kurz := err.Error()
	if i := strings.IndexByte(kurz, '\n'); i > 0 {
		kurz = kurz[:i]
	}
	if pfad, suchfehler := exec.LookPath("notify-send"); suchfehler == nil {
		_ = exec.Command(pfad, "-u", "critical", "Esstracker startet nicht", kurz).Run()
	}
	return err
}

// erreichbar prueft, ob dort schon ein Esstracker antwortet.
func erreichbar(adr string) bool {
	klient := &http.Client{Timeout: 900 * time.Millisecond}
	antwort, err := klient.Get("http://" + adr + "/api/stand")
	if err != nil {
		return false
	}
	defer antwort.Body.Close()
	return antwort.StatusCode == http.StatusOK
}

// ---------- Dienst ----------

func dienst() error {
	buch, empfang, berater, schaetzer, planer, err := teile()
	if err != nil {
		return err
	}
	adr := adresse()
	server := &http.Server{
		Addr:              adr,
		Handler:           web.Neu(buch, empfang, berater, schaetzer, planer).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	schaetzenAn(empfang, schaetzer)
	ctx, halt := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer halt()

	empfangZu := empfangHoch(empfang, true)

	go func() {
		fmt.Println("Oberfläche: http://" + adr + "/")
		fmt.Println("Empfang:    " + empfangsadresse() + "  (Adressen: " + strings.Join(aufnahme.Adressen(), ", ") + ")")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "Server: "+err.Error())
		}
	}()
	<-ctx.Done()
	empfangZu()
	aus, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	return server.Shutdown(aus)
}

// ---------- Terminal ----------

// heute zeigt denselben Stand wie oben im Fenster, nur als Text — fuer den
// Fall, dass gerade ein Terminal offen ist und kein Fenster.
func heute() error {
	buch, err := tagebuch.Oeffne()
	if err != nil {
		return err
	}
	s := buch.Sicht()
	z := s.Zahlen
	fmt.Printf("%s — %d von %d kcal (%d%%), noch %d\n\n", s.Heute, z.Kcal, z.Ziel, z.Prozent, z.Rest)
	for _, t := range s.Tage {
		if t.Tag != s.Heute {
			continue
		}
		for _, r := range t.Zeilen {
			kcal := fmt.Sprintf("%5d", r.Kcal)
			if r.Quelle == tagebuch.QuelleOffen {
				kcal = "    ?" // noch nicht geschaetzt — eine 0 waere gelogen
			}
			fmt.Printf("  %s  %-34s %6.0f %-2s  %s kcal\n", r.Uhr, kurz(r.Name, 34), r.Menge, r.Einheit, kcal)
		}
	}
	if z.Eintraege == 0 {
		fmt.Println("  (noch nichts eingetragen)")
	}
	n := z.Naehrwerte
	fmt.Printf("\n  Fett %.0f g · Kohlenhydrate %.0f g (Zucker %.0f g) · Eiweiß %.0f g · Salz %.1f g\n",
		n.Fett, n.Kohlenhydrate, n.Zucker, n.Eiweiss, n.Salz)
	return nil
}

func kurz(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// koppeln oeffnet das Fenster vom Terminal aus und wartet, bis ein Handy
// durchkommt — der Weg fuer den Fall, dass gerade kein Fenster offen ist.
func koppeln() error {
	buch, empfang, _, _, _, err := teile()
	if err != nil {
		return err
	}
	if erreichbar(adresse()) {
		return errors.New("es läuft schon ein Esstracker — koppeln geht dort im Fenster unter „Handy\"")
	}
	empfangZu := empfangHoch(empfang, true)
	defer empfangZu()

	wort := empfang.Oeffnen()
	vorher := len(buch.Sicht().Geraete)

	fmt.Println("Kopplung offen für " + aufnahme.KopplungDauer.String() + ".")
	fmt.Println("Erkennungswort:  " + wort)
	fmt.Println("Adressen:        " + strings.Join(aufnahme.Adressen(), ", ") + fmt.Sprintf(" (Port %d)", aufnahme.PortAus(empfangsadresse())))
	fmt.Println("\nAm Handy: Einstellungen → Rechner suchen. Abbruch mit Strg-C.")

	ctx, halt := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer halt()
	uhr := time.NewTicker(time.Second)
	defer uhr.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-uhr.C:
			geraete := buch.Sicht().Geraete
			if len(geraete) > vorher {
				fmt.Println("\n→ gekoppelt: " + geraete[len(geraete)-1].Name)
				return nil
			}
			if !empfang.Stand().Offen {
				return errors.New("Kopplungsfenster ist abgelaufen, ohne dass ein Handy kam")
			}
		}
	}
}

// ausfuhr schreibt alles als CSV auf die Standardausgabe — fuer `esstracker
// ausfuhr > tabelle.csv` und fuer alles, was danach eine Pipe kommt.
func ausfuhr() error {
	buch, err := tagebuch.Oeffne()
	if err != nil {
		return err
	}
	schreiber := csv.NewWriter(os.Stdout)
	if err := schreiber.WriteAll(web.Tabelle(buch.Sicht())); err != nil {
		return err
	}
	return nil
}

// ---------- Einrichten ----------

// einrichten legt Starter und Symbol im Anwendungsmenue ab. Fuer dmenu_run
// braucht es das nicht — das liest den PATH —, aber ein Programm ohne Eintrag
// im Menue ist fuer jeden anderen Starter unsichtbar.
func einrichten() error {
	pfad, err := os.Executable()
	if err != nil {
		return err
	}
	heim, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ordner := heim + "/.local/share/applications"
	if err := os.MkdirAll(ordner, 0o755); err != nil {
		return err
	}
	symbolOrdner := heim + "/.local/share/icons/hicolor/scalable/apps"
	if err := os.MkdirAll(symbolOrdner, 0o755); err != nil {
		return err
	}
	symbol := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <rect width="128" height="128" rx="28" fill="#0A0A0B"/>
  <path d="M40 30v30a12 12 0 0 0 24 0V30" stroke="#FAFAFA" stroke-width="7"
        fill="none" stroke-linecap="round"/>
  <path d="M52 30v68" stroke="#FAFAFA" stroke-width="7" stroke-linecap="round"/>
  <path d="M88 30c-8 0-12 10-12 22s4 16 12 16" stroke="#FAFAFA" stroke-width="7"
        fill="none" stroke-linecap="round" stroke-linejoin="round"/>
  <path d="M88 30v68" stroke="#FAFAFA" stroke-width="7" stroke-linecap="round"/>
</svg>
`
	if err := os.WriteFile(symbolOrdner+"/esstracker.svg", []byte(symbol), 0o644); err != nil {
		return err
	}
	starter := `[Desktop Entry]
Type=Application
Name=Esstracker
GenericName=Ernährungstagebuch
Comment=Was gegessen wurde, vom Handy auf diesen Rechner
Exec=` + pfad + `
Icon=esstracker
Terminal=false
Categories=Utility;Office;
StartupNotify=true
StartupWMClass=esstracker
`
	if err := os.WriteFile(ordner+"/esstracker.desktop", []byte(starter), 0o644); err != nil {
		return err
	}
	fmt.Println("geschrieben: " + ordner + "/esstracker.desktop")
	fmt.Println("geschrieben: " + symbolOrdner + "/esstracker.svg")
	return nil
}
