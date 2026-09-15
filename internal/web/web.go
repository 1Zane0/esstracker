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
	"github.com/1Zane0/esstracker/internal/tagebuch"
)

//go:embed dateien
var dateien embed.FS

// Dienst haelt alles, was die Oberflaeche braucht.
type Dienst struct {
	Buch    *tagebuch.Buch
	Empfang *aufnahme.Dienst
}

// Neu baut den Dienst.
func Neu(b *tagebuch.Buch, e *aufnahme.Dienst) *Dienst { return &Dienst{Buch: b, Empfang: e} }

// Handler verdrahtet alle Pfade.
func (d *Dienst) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/stand", d.stand)
	mux.HandleFunc("POST /api/ziel", d.ziel)
	mux.HandleFunc("DELETE /api/eintrag/{id}", d.eintragLoeschen)
	mux.HandleFunc("POST /api/kopplung/auf", d.kopplungAuf)
	mux.HandleFunc("POST /api/kopplung/zu", d.kopplungZu)
	mux.HandleFunc("DELETE /api/geraet/{id}", d.geraetLoesen)
	mux.HandleFunc("GET /api/ausfuhr.csv", d.ausfuhr)
	mux.HandleFunc("POST /api/ausfuhr", d.ausfuhrDatei)

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
