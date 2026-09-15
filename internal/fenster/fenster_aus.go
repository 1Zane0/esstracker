//go:build ohne_fenster

package fenster

import "fmt"

// Verfuegbar meldet, ob dieses Binary ein Fenster oeffnen kann.
const Verfuegbar = false

// Oeffne gibt es in dieser Fassung nicht — sie ist ohne WebKitGTK uebersetzt,
// damit sie auf einem Server ohne Grafikbibliotheken baut.
func Oeffne(url, titel string) error {
	return fmt.Errorf("diese Fassung ist ohne Fenster gebaut (-tags ohne_fenster).\n"+
		"Nimm `esstracker dienst` und ruf %s im Browser auf.", url)
}
