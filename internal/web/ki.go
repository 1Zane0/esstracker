package web

import (
	"encoding/json"
	"net/http"

	"github.com/1Zane0/esstracker/internal/ki"
)

// Der Reiter „KI" folgt demselben Grundsatz wie der Rest: jede Anfrage — auch
// jede aendernde — antwortet mit dem vollstaendigen Stand des Gespraechs, und
// die Oberflaeche zeichnet ihn neu. Nur ist es hier ein eigener Stand und
// nicht der des Tagebuchs: waehrend eine Antwort entsteht, fragt das Fenster
// im Sekundentakt nach, und dafuer waere das Jahresraster jedes Mal mitzubauen
// reine Arbeit ohne Ertrag.

func (d *Dienst) sendeKI(w http.ResponseWriter) {
	g, lauf := d.Berater.Hole()
	bereit, hinweis := ki.Bereit()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"gespraech": g,
		"laeuft":    lauf != nil,
		"lauf":      lauf,
		"bereit":    bereit,
		"hinweis":   hinweis,
	})
}

// fertigKI ist die Antwort auf jede Aenderung: entweder der Fehler oder der
// neue Stand.
func (d *Dienst) fertigKI(w http.ResponseWriter, err error) {
	if err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	d.sendeKI(w)
}

func (d *Dienst) kiLesen(w http.ResponseWriter, r *http.Request) {
	d.sendeKI(w)
}

func (d *Dienst) kiFragen(w http.ResponseWriter, r *http.Request) {
	var wu struct {
		Text string `json:"text"`
	}
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	d.fertigKI(w, d.Berater.Fragen(wu.Text))
}

func (d *Dienst) kiNeu(w http.ResponseWriter, r *http.Request) {
	d.fertigKI(w, d.Berater.Neu())
}

func (d *Dienst) kiVergessen(w http.ResponseWriter, r *http.Request) {
	d.fertigKI(w, d.Berater.Vergessen())
}

// kiAbbruch ist die Notbremse. Sie antwortet mit dem Stand von jetzt — der
// laufende Prozess braucht danach noch einen Moment, bis er wirklich steht,
// und genau das soll das Fenster zeigen.
func (d *Dienst) kiAbbruch(w http.ResponseWriter, r *http.Request) {
	d.fertigKI(w, d.Berater.Abbrechen())
}
