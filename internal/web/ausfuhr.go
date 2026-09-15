package web

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/1Zane0/esstracker/internal/speicher"
	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// Die Ausfuhr gibt es zweimal, und das hat einen Grund.
//
// `GET /api/ausfuhr.csv` ist der Weg fuer alles ausserhalb des Fensters —
// curl, ein Browser, ein Skript. Im Fenster selbst hilft das nicht: WebKitGTK
// hat hier keinen Download-Weg, ein Link mit `download` tut schlicht nichts.
// Deshalb legt `POST /api/ausfuhr` die Datei selbst ab und meldet den Pfad
// zurueck, den die Oberflaeche dann anzeigen kann.

var spalten = []string{
	"tag", "zeit", "mahlzeit", "name", "marke", "barcode",
	"menge", "einheit", "kcal", "kj", "fett", "gesaettigt",
	"kohlenhydrate", "zucker", "ballaststoffe", "eiweiss", "salz",
	"kcal_je_100", "quelle", "geraet", "id",
}

// Tabelle baut die Tabelle: eine Zeile je Eintrag, die Naehrwerte auf die
// gegessene Menge hochgerechnet — das ist die Zahl, nach der jemand sucht,
// der die Datei in eine Tabellenkalkulation zieht.
func Tabelle(s tagebuch.Sicht) [][]string {
	raus := [][]string{spalten}
	// Aelteste zuerst: eine Tabelle liest man von oben nach unten durch die
	// Zeit, anders als die Oberflaeche, die den heutigen Tag oben haben will.
	for i := len(s.Tage) - 1; i >= 0; i-- {
		for _, z := range s.Tage[i].Zeilen {
			g := z.Gesamt
			raus = append(raus, []string{
				z.Tag, z.Zeit, z.Mahlzeit, z.Name, z.Marke, z.Barcode,
				zahl(z.Menge), z.Einheit,
				zahl(g.Kcal), zahl(g.KJ), zahl(g.Fett), zahl(g.Gesaettigt),
				zahl(g.Kohlenhydrate), zahl(g.Zucker), zahl(g.Ballaststoffe),
				zahl(g.Eiweiss), zahl(g.Salz),
				zahl(z.Je100.Kcal), z.Quelle, z.Geraet, z.ID,
			})
		}
	}
	return raus
}

// zahl schreibt ohne unnoetige Nullen und auf zwei Stellen gerundet.
//
// Ohne das Runden stehen in der Tabelle Zahlen wie 0.020999999999999998 — das
// ist keine Genauigkeit, sondern der Rest einer Multiplikation im
// Binaerformat. Zwei Stellen sind mehr, als eine Naehrwertangabe hergibt: in
// der Datenbank steht 0,02 g Salz je 100 g, nicht 0,0209999.
func zahl(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}

func (d *Dienst) ausfuhr(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="esstracker.csv"`)
	schreiber := csv.NewWriter(w)
	_ = schreiber.WriteAll(Tabelle(d.Buch.Sicht()))
}

// ausfuhrDatei legt die Tabelle neben dem Tagebuch ab.
func (d *Dienst) ausfuhrDatei(w http.ResponseWriter, r *http.Request) {
	name := "esstracker-" + time.Now().Format("2006-01-02") + ".csv"
	pfad := speicher.Pfad(name)

	if err := os.MkdirAll(speicher.Ordner(), 0o700); err != nil {
		fehler(w, http.StatusInternalServerError, err)
		return
	}
	datei, err := os.Create(pfad)
	if err != nil {
		fehler(w, http.StatusInternalServerError, err)
		return
	}
	schreiber := csv.NewWriter(datei)
	schreibfehler := schreiber.WriteAll(Tabelle(d.Buch.Sicht()))
	schliessfehler := datei.Close()
	if schreibfehler != nil {
		fehler(w, http.StatusInternalServerError, schreibfehler)
		return
	}
	if schliessfehler != nil {
		fehler(w, http.StatusInternalServerError, schliessfehler)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"pfad":%q}`, pfad)
}
