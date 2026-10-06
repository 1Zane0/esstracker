// Package web liefert die Oberflaeche aus und beantwortet ihre Anfragen.
//
// Der Dienst hat bewusst keine Anmeldung: er haengt an der Loopback-Adresse
// und darf damit genau das, was der angemeldete Benutzer ohnehin darf. Alles,
// was vom Heimnetz aus erreichbar sein muss, steht nebenan in `aufnahme` —
// diese Trennung ist der Grund, warum es hier kein Token gibt.
//
// Wie in der Tasklist nebenan antwortet jede Anfrage — auch jede aendernde —
// mit dem vollstaendigen Stand. Die Oberflaeche muss deshalb nie selbst
// mitrechnen, was ein geloeschter Eintrag an Tagessummen und Raster
// verschiebt. Sie zeichnet nur, was zurueckkommt.
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"

	"github.com/1Zane0/esstracker/internal/aufnahme"
	"github.com/1Zane0/esstracker/internal/ki"
	"github.com/1Zane0/esstracker/internal/tagebuch"
)

//go:embed dateien
var dateien embed.FS

// Dienst haelt alles, was die Oberflaeche braucht.
type Dienst struct {
	Buch      *tagebuch.Buch
	Empfang   *aufnahme.Dienst
	Berater   *ki.Berater
	Schaetzer *ki.Schaetzer
	Planer    *ki.Planer
}

// Neu baut den Dienst.
func Neu(b *tagebuch.Buch, e *aufnahme.Dienst, k *ki.Berater, s *ki.Schaetzer, p *ki.Planer) *Dienst {
	return &Dienst{Buch: b, Empfang: e, Berater: k, Schaetzer: s, Planer: p}
}

// Handler verdrahtet alle Pfade.
func (d *Dienst) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/stand", d.stand)
	mux.HandleFunc("POST /api/ziel", d.ziel)
	mux.HandleFunc("DELETE /api/eintrag/{id}", d.eintragLoeschen)
	mux.HandleFunc("POST /api/eintrag/{id}/schaetzen", d.eintragSchaetzen)
	mux.HandleFunc("POST /api/schaetzen", d.schaetzen)
	mux.HandleFunc("POST /api/schaetzen/abbruch", d.schaetzenAbbruch)
	mux.HandleFunc("DELETE /api/eigene/{id}", d.eigenesLoeschen)
	mux.HandleFunc("POST /api/kopplung/auf", d.kopplungAuf)
	mux.HandleFunc("POST /api/kopplung/zu", d.kopplungZu)
	mux.HandleFunc("DELETE /api/geraet/{id}", d.geraetLoesen)
	mux.HandleFunc("POST /api/einkauf/erstellen", d.einkaufErstellen)
	mux.HandleFunc("POST /api/einkauf/abbruch", d.einkaufAbbruch)
	mux.HandleFunc("POST /api/einkauf/posten", d.postenAnlegen)
	mux.HandleFunc("POST /api/einkauf/posten/{id}/haken", d.postenHaken)
	mux.HandleFunc("POST /api/einkauf/posten/{id}/schritt", d.postenSchritt)
	mux.HandleFunc("POST /api/einkauf/posten/{id}/menge", d.postenMenge)
	mux.HandleFunc("DELETE /api/einkauf/posten/{id}", d.postenLoeschen)
	mux.HandleFunc("POST /api/einkauf/aufraeumen", d.einkaufAufraeumen)
	mux.HandleFunc("POST /api/einkauf/leeren", d.einkaufLeeren)

	mux.HandleFunc("GET /api/ausfuhr.csv", d.ausfuhr)
	mux.HandleFunc("POST /api/ausfuhr", d.ausfuhrDatei)

	mux.HandleFunc("GET /api/ki", d.kiLesen)
	mux.HandleFunc("POST /api/ki/frage", d.kiFragen)
	mux.HandleFunc("POST /api/ki/neu", d.kiNeu)
	mux.HandleFunc("POST /api/ki/vergessen", d.kiVergessen)
	mux.HandleFunc("POST /api/ki/abbruch", d.kiAbbruch)

	unter, err := fs.Sub(dateien, "dateien")
	if err != nil {
		panic("eingebettete Dateien fehlen: " + err.Error())
	}
	mux.Handle("/", kopfzeilen(http.FileServer(http.FS(unter))))

	return mux
}

// kopfzeilen verbietet den Zwischenspeicher. Im Fenster laeuft dieselbe
// WebKit-Instanz oft tagelang; ohne das zeigt sie nach einem Neubau die alte
// Oberflaeche und man sucht den Fehler im Programm.
func kopfzeilen(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// ---------- Antworten ----------

func (d *Dienst) sendeStand(w http.ResponseWriter) {
	s := d.Buch.Sicht()
	s.Kopplung = d.Empfang.Stand()
	s.Schaetzung = d.Schaetzer.Stand()
	s.Plan = d.Planer.Stand()
	json.NewEncoder(w).Encode(s)
}

// fertig ist die Antwort auf jede Aenderung: entweder der Fehler oder der
// neue Stand.
func (d *Dienst) fertig(w http.ResponseWriter, err error) {
	if err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	d.sendeStand(w)
}

func fehler(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"fehler": err.Error()})
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

// ---------- Pfade ----------

func (d *Dienst) stand(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	d.sendeStand(w)
}

type wunsch struct {
	Kcal int `json:"kcal"`
}

func (d *Dienst) ziel(w http.ResponseWriter, r *http.Request) {
	var wu wunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	d.fertig(w, d.Buch.ZielSetzen(wu.Kcal))
}

func (d *Dienst) eintragLoeschen(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Buch.Loeschen(r.PathValue("id")))
}

// eintragSchaetzen gibt einen selbst eingetippten Eintrag zum Schaetzen frei —
// fuer die 1 kcal, die am Handy nur stand, weil das Feld nicht leer bleiben
// durfte.
func (d *Dienst) eintragSchaetzen(w http.ResponseWriter, r *http.Request) {
	if err := d.Buch.Freigeben(r.PathValue("id")); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	d.Schaetzer.Anstossen(true)
	d.fertig(w, nil)
}

func (d *Dienst) schaetzen(w http.ResponseWriter, r *http.Request) {
	d.Schaetzer.Anstossen(true)
	d.fertig(w, nil)
}

func (d *Dienst) schaetzenAbbruch(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Schaetzer.Abbrechen())
}

func (d *Dienst) eigenesLoeschen(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Buch.EigenesLoeschen(r.PathValue("id")))
}

func (d *Dienst) kopplungAuf(w http.ResponseWriter, r *http.Request) {
	d.Empfang.Oeffnen()
	d.fertig(w, nil)
}

func (d *Dienst) kopplungZu(w http.ResponseWriter, r *http.Request) {
	d.Empfang.Schliessen()
	d.fertig(w, nil)
}

func (d *Dienst) geraetLoesen(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Buch.GeraetLoesen(r.PathValue("id")))
}
