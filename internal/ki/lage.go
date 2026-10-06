package ki

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/1Zane0/esstracker/internal/tagebuch"
)

// anweisung steht als Systemtext vor jeder Sitzung. Sie sagt drei Dinge: was
// hier vorliegt, was nicht erfunden werden darf, und wo die Grenze zur Praxis
// verlaeuft.
const anweisung = `Du bist die Auswertung im Esstracker — einem Essenstagebuch, das auf diesem
Rechner läuft. Eingetragen wird am Handy, gesammelt wird hier.

Vor jeder Frage steht ein <stand>-Block: der heutige Tag mit jedem einzelnen
Eintrag, die Tage davor als Zeilen. Der Block kommt vom Programm, nicht vom
Menschen. Steht im Verlauf mehr als einer, gilt der letzte — die früheren sind
überholt.

Rechne mit den Zahlen, die dastehen, und erfinde keine dazu. Woher sie kommen,
steht in der Spalte Quelle:

- DB: aus Open Food Facts, einer Sammlung von Freiwilligen. Fehlt dort eine
  Angabe, steht eine Null — sag dann, dass sie fehlt, statt sie zu schätzen.
- selbst: am Handy von Hand eingetippt.
- geschätzt: am Handy stand nur Name und Menge, die Nährwerte hat danach ein
  Modell geschätzt. Nenn solche Zahlen geschätzt, wenn es auf sie ankommt.
- eigene DB: am Handy aus der eigenen Datenbank gewählt. Dort liegt, was
  früher schon einmal geschätzt wurde — auch diese Zahlen sind Schätzungen.
- offen: noch ohne Nährwerte, die Schätzung steht aus. Der Eintrag zählt
  vorläufig mit null — die Tagessumme ist dann zu niedrig, und das gehört in
  die Antwort.

Im Arbeitsordner liegt tagebuch.json mit dem vollständigen Verlauf. Geht eine
Frage über die angehängten Tage hinaus, lies dort nach. Ändern kannst du
nichts: ein Eintrag beschreibt ein Ereignis in der Vergangenheit, und
korrigiert wird im Fenster.

Antworte auf Deutsch und fang mit der Antwort an — keine Einleitung, keine
Wiederholung der Frage, kein Rückblick auf das, was du gerade getan hast.
Zahlen mit Einheit. Lieber ein Satz zu wenig als ein Absatz zu viel.

Du bist kein Arzt. Einordnen, rechnen, Muster zeigen: ja. Diagnosen, Therapien,
Medikamente, Verdachtsäußerungen zu Krankheiten: nein — dafür in eine Praxis
verweisen.

Markdown wird gezeichnet: ## Überschriften, Listen, **fett**, ` + "`Code`" + ` und
Tabellen mit |. Bilder und Links nicht.`

var wochentage = [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

var mahlzeiten = map[string]string{
	"fruehstueck": "Frühstück", "mittag": "Mittag", "abend": "Abend", "snack": "Zwischendurch",
}

// TageImBlock ist, wie viele vergangene Tage mit angehaengt werden. Alles
// darueber steht in tagebuch.json und wird bei Bedarf dort gelesen — der Block
// soll den Tag beschreiben, nicht das Archiv mitschleppen.
const TageImBlock = 21

// ProdukteImBlock deckelt die Produktliste aus demselben Grund.
const ProdukteImBlock = 15

// lage schreibt den Stand des Tagebuchs auf. Er wird vor jede Frage gehaengt,
// nicht einmal zu Beginn der Sitzung: waehrend eine Unterhaltung laeuft, kommen
// vom Handy weiter Eintraege herein, und die Antwort auf "und jetzt?" soll die
// Zahlen von jetzt meinen.
func lage(s tagebuch.Sicht) string {
	var b strings.Builder

	jetzt := time.Now()
	fmt.Fprintf(&b, "<stand abgerufen=\"%s, %s\">\n\n", wochentag(s.Heute), jetzt.Format("02.01.2006 15:04"))
	fmt.Fprintf(&b, "Tagesziel: %d kcal.\n\n", s.Ziel)

	heuteBlock(&b, s)
	tageBlock(&b, s)
	produkteBlock(&b, s)

	fmt.Fprintf(&b, "\nDer vollständige Verlauf steht in tagebuch.json im Arbeitsordner.\n")
	b.WriteString("</stand>\n")
	return b.String()
}

func heuteBlock(b *strings.Builder, s tagebuch.Sicht) {
	z := s.Zahlen
	fmt.Fprintf(b, "## Heute — %s, %s\n\n", wochentag(s.Heute), deutsch(s.Heute))

	if z.Eintraege == 0 {
		b.WriteString("Heute ist noch nichts eingetragen.\n\n")
		return
	}

	rest := "%d kcal übrig"
	uebrig := z.Rest
	if uebrig < 0 {
		rest, uebrig = "%d kcal über dem Ziel", -uebrig
	}
	fmt.Fprintf(b, "%d kcal (%d %% vom Ziel, "+rest+") · %s · %s\n\n",
		z.Kcal, z.Prozent, uebrig, naehrzeile(z.Naehrwerte), anzahl(z.Eintraege, "Eintrag", "Einträge"))

	var heute *tagebuch.Tag
	for i := range s.Tage {
		if s.Tage[i].Tag == s.Heute {
			heute = &s.Tage[i]
			break
		}
	}
	if heute == nil {
		return
	}

	b.WriteString("| Zeit | Mahlzeit | Produkt | Marke | Menge | kcal | Fett | KH | Zucker | Ballast | Eiweiß | Salz | Quelle |\n")
	b.WriteString("|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	offen := 0
	for _, z := range heute.Zeilen {
		g := z.Gesamt
		kcal := strconv.Itoa(z.Kcal)
		if z.Quelle == tagebuch.QuelleOffen {
			kcal = "?"
			offen++
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			z.Uhr, mahlzeit(z.Mahlzeit), feld(z.Name), feld(z.Marke),
			gr(z.Menge), z.Einheit, kcal,
			gr(g.Fett), gr(g.Kohlenhydrate), gr(g.Zucker),
			gr(g.Ballaststoffe), gr(g.Eiweiss), gr(g.Salz), quelle(z.Quelle))
	}
	b.WriteString("\n")
	if offen > 0 {
		fmt.Fprintf(b, "%s ohne Nährwerte — die Summe oben ist deshalb zu niedrig.\n\n", anzahl(offen, "Eintrag", "Einträge"))
	}
}

// quelle schreibt die Herkunft so, wie der Systemtext sie erklaert.
func quelle(q string) string {
	switch q {
	case tagebuch.QuelleHand:
		return "selbst"
	case tagebuch.QuelleKI:
		return "geschätzt"
	case tagebuch.QuelleEigen:
		return "eigene DB"
	case tagebuch.QuelleOffen:
		return "offen"
	}
	return "DB"
}

func tageBlock(b *strings.Builder, s tagebuch.Sicht) {
	// Der heutige Tag steht schon oben, in voller Aufloesung.
	var aeltere []tagebuch.Tag
	for _, t := range s.Tage {
		if t.Tag == s.Heute {
			continue
		}
		aeltere = append(aeltere, t)
		if len(aeltere) == TageImBlock {
			break
		}
	}
	if len(aeltere) == 0 {
		b.WriteString("## Die Tage davor\n\nKeine älteren Tage im Tagebuch.\n\n")
		return
	}

	fmt.Fprintf(b, "## Die Tage davor (die letzten %s mit Einträgen)\n\n", anzahl(len(aeltere), "Tag", "Tage"))
	b.WriteString("| Tag | kcal | % vom Ziel | Einträge | Fett | KH | Zucker | Ballast | Eiweiß | Salz |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, t := range aeltere {
		n := t.Naehrwerte
		fmt.Fprintf(b, "| %s, %s | %d | %d | %d | %s | %s | %s | %s | %s | %s |\n",
			kurzerTag(t.Tag), deutsch(t.Tag), t.Kcal, t.Prozent, len(t.Zeilen),
			gr(n.Fett), gr(n.Kohlenhydrate), gr(n.Zucker),
			gr(n.Ballaststoffe), gr(n.Eiweiss), gr(n.Salz))
	}

	// Leere Tage zaehlen im Schnitt nicht mit: wer zwei Tage nichts
	// eingetragen hat, hat nicht zwei Tage nichts gegessen.
	fmt.Fprintf(b, "\nSchnitt der letzten sieben Tage mit Einträgen: %d kcal. Getrackte Tage insgesamt: %d.\n"+
		"Tage ohne Eintrag stehen nicht in der Tabelle — an ihnen wurde nichts eingetragen, nicht nichts gegessen.\n\n",
		s.Zahlen.Schnitt7, s.Zahlen.Tage)
}

func produkteBlock(b *strings.Builder, s tagebuch.Sicht) {
	if len(s.Produkte) == 0 {
		return
	}
	liste := s.Produkte
	if len(liste) > ProdukteImBlock {
		liste = liste[:ProdukteImBlock]
	}
	fmt.Fprintf(b, "## Häufigste Produkte (%d von %d)\n\n", len(liste), len(s.Produkte))
	b.WriteString("| Produkt | Marke | mal | kcal je 100 | insgesamt | zuletzt |\n")
	b.WriteString("|---|---|---:|---:|---:|---|\n")
	for _, p := range liste {
		einheit := p.Einheit
		if einheit == "" {
			einheit = "g"
		}
		fmt.Fprintf(b, "| %s | %s | %d | %s | %s %s | %s |\n",
			feld(p.Name), feld(p.Marke), p.Anzahl, gr(p.Kcal100), gr(p.Menge), einheit, deutsch(p.Zuletzt))
	}
	b.WriteString("\n")
}

// ---------- Kleinkram ----------

// gr schreibt Gramm so, wie das Fenster sie schreibt: ab zehn ohne
// Nachkommastelle, darunter mit einer — 0,8 g Salz soll nicht zu 1 g werden.
func gr(f float64) string {
	if f == 0 {
		return "0"
	}
	stellen := 1
	if f >= 10 || f <= -10 {
		stellen = 0
	}
	return strings.Replace(strconv.FormatFloat(f, 'f', stellen, 64), ".", ",", 1)
}

func naehrzeile(n tagebuch.Naehrwerte) string {
	return fmt.Sprintf("Fett %s g · Kohlenhydrate %s g · Zucker %s g · Ballaststoffe %s g · Eiweiß %s g · Salz %s g",
		gr(n.Fett), gr(n.Kohlenhydrate), gr(n.Zucker), gr(n.Ballaststoffe), gr(n.Eiweiss), gr(n.Salz))
}

func anzahl(n int, eins, viele string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, eins)
	}
	return fmt.Sprintf("%d %s", n, viele)
}

func mahlzeit(s string) string {
	if name, da := mahlzeiten[s]; da {
		return name
	}
	return s
}

// feld haelt einen Produktnamen aus einer fremden Datenbank aus der
// Tabellenform heraus: ein Senkrechtstrich im Namen wuerde sonst eine Spalte
// erfinden.
func feld(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "|", "/"))
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "—"
	}
	return kuerzen(s, 60)
}

func datum(tag string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", tag)
	return t, err == nil
}

func wochentag(tag string) string {
	t, ok := datum(tag)
	if !ok {
		return tag
	}
	return wochentage[int(t.Weekday())]
}

func kurzerTag(tag string) string {
	name := wochentag(tag)
	if len(name) < 2 {
		return name
	}
	return name[:2]
}

func deutsch(tag string) string {
	t, ok := datum(tag)
	if !ok {
		return tag
	}
	return t.Format("02.01.2006")
}
