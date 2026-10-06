package web

import (
	"errors"
	"net/http"

	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// Die Einkaufsliste folgt demselben Grundsatz wie der Rest der Oberflaeche:
// jede Anfrage — auch jede aendernde — antwortet mit dem vollstaendigen Stand,
// und das Fenster zeichnet ihn neu. Das ist hier mehr als Bequemlichkeit: die
// Liste aendert sich auch von aussen, wenn im Laden jemand am Handy abhakt, und
// eine Oberflaeche, die nur ihre eigene Aenderung nachzieht, zeigt dann eine
// Liste, die es so nicht mehr gibt.

type listenWunsch struct {
	Wunsch    string `json:"wunsch"`
	Ergaenzen bool   `json:"ergaenzen"`
}

// einkaufErstellen laesst Claude schreiben. Die Antwort kommt sofort, mit dem
// Stand von jetzt: die Liste steht ein paar Sekunden spaeter da, und bis dahin
// zeigt das Fenster, dass etwas laeuft.
func (d *Dienst) einkaufErstellen(w http.ResponseWriter, r *http.Request) {
	var wu listenWunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	d.fertig(w, d.Planer.Erstellen(wu.Wunsch, wu.Ergaenzen))
}

func (d *Dienst) einkaufAbbruch(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Planer.Abbrechen())
}

type postenWunsch struct {
	Name    string  `json:"name"`
	Menge   float64 `json:"menge"`
	Einheit string  `json:"einheit"`
	Gruppe  string  `json:"gruppe"`
}

// postenAnlegen ist das Plus unter der Liste: was Claude nicht wusste, kommt
// von Hand dazu.
func (d *Dienst) postenAnlegen(w http.ResponseWriter, r *http.Request) {
	var wu postenWunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	_, err := d.Buch.PostenAnlegen(tagebuch.Posten{
		Name: wu.Name, Menge: wu.Menge, Einheit: wu.Einheit, Gruppe: wu.Gruppe,
	})
	d.fertig(w, err)
}

type hakenWunsch struct {
	Erledigt bool `json:"erledigt"`
}

func (d *Dienst) postenHaken(w http.ResponseWriter, r *http.Request) {
	var wu hakenWunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	// Ohne Geraet und ohne Zeit: beides setzt das Tagebuch auf „hier, jetzt".
	d.fertig(w, d.Buch.PostenHaken(r.PathValue("id"), wu.Erledigt, "", ""))
}

type schrittWunsch struct {
	Richtung int `json:"richtung"` // +1 oder -1
}

// postenSchritt ist das, was die Knoepfe + und − tun. Um wie viel es geht,
// entscheidet die Einheit im Tagebuch und nicht die Oberflaeche: sonst gaebe
// das Plus am Handy einen anderen Schritt als das im Fenster.
func (d *Dienst) postenSchritt(w http.ResponseWriter, r *http.Request) {
	var wu schrittWunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	if wu.Richtung == 0 {
		fehler(w, http.StatusBadRequest, errors.New("Richtung fehlt"))
		return
	}
	_, err := d.Buch.PostenSchritt(r.PathValue("id"), wu.Richtung, "")
	d.fertig(w, err)
}

type mengeWunsch struct {
	Menge   float64 `json:"menge"`
	Einheit string  `json:"einheit"`
}

// postenMenge setzt eine eingetippte Zahl oder tauscht die Einheit.
func (d *Dienst) postenMenge(w http.ResponseWriter, r *http.Request) {
	var wu mengeWunsch
	if err := lies(r, &wu); err != nil {
		fehler(w, http.StatusBadRequest, err)
		return
	}
	d.fertig(w, d.Buch.PostenMenge(r.PathValue("id"), wu.Menge, wu.Einheit, "", ""))
}

func (d *Dienst) postenLoeschen(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Buch.PostenLoeschen(r.PathValue("id")))
}

func (d *Dienst) einkaufAufraeumen(w http.ResponseWriter, r *http.Request) {
	_, err := d.Buch.EinkaufAufraeumen()
	d.fertig(w, err)
}

func (d *Dienst) einkaufLeeren(w http.ResponseWriter, r *http.Request) {
	d.fertig(w, d.Buch.EinkaufLeeren())
}
