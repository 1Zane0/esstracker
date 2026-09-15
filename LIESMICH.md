# Esstracker

Was gegessen wurde — auf dem Handy erfasst, auf diesem Rechner gesammelt.

Ein Tracker, eine Zahl: das Handy zeigt die Kalorien von heute und sonst
nichts. Übertragen wird trotzdem alles — Produkt, Marke, Strichcode, Menge,
Uhrzeit, Mahlzeit und der volle Nährwertsatz. Die vollständige Ansicht steht
hier auf dem Rechner, und von dort geht alles als CSV weiter.

Aufbau bewusst wie [tasklist](../tasklist) und [wachturm](../wachturm): Go,
WebKitGTK-Fenster, Oberfläche per `go:embed`, schwarz-weiß, dieselbe Schrift
und dieselben Farbtoken. Die App ist auf dieselben Werte gesetzt.

## Zwei Türen

| Was | Adresse | Für wen |
|---|---|---|
| Oberfläche | `127.0.0.1:8789` | nur dieser Rechner |
| Empfang (TCP) | `0.0.0.0:8790` | das gekoppelte Handy im Heimnetz |
| Suche (UDP) | `0.0.0.0:8790` | das Handy, damit es die Adresse nicht kennen muss |

8789 liegt neben der Tasklist auf 8788 und dem Wachturm auf 8787 — alle drei
dürfen gleichzeitig offen sein.

Die Trennung ist der Entwurf: die Oberfläche hat keine Anmeldung und braucht
auch keine, weil sie vom Netz aus nicht erreichbar ist. Draußen steht nur, was
ein Handy wirklich braucht.

## Kopplung

Ohne Tipparbeit und trotzdem nicht offen:

1. Am Rechner: **Handy → Kopplung öffnen**. Das Fenster steht fünf Minuten.
2. Am Handy: **Rechner → Rechner suchen**. Es findet den Rechner über einen
   UDP-Rundruf von allein.
3. Das Erkennungswort steht auf beiden Seiten — vergleichen, koppeln, fertig.

Ein Token bekommt nur, wer sich meldet, während das Fenster offen steht; mit
dem ersten Gerät schließt es sich. Der gefährliche Zustand ist damit kein
Dauerzustand, sondern eine Ausnahme, die jemand absichtlich herbeiführt und
sieht. Wer währenddessen im selben Heimnetz mitliest, kann sich dazwischen
drängen — das ist der Preis, und er ist hier benannt.

Kommt der Rundruf nicht durch (manche Router sperren ihn), steht die Adresse
am Rechner unter „Handy" und lässt sich am Handy von Hand eintragen.

## Bauen

```sh
bin/bau.sh                # Programm mit Fenster
bin/bau.sh --ohne-fenster # nur der Dienst, ohne GTK/WebKit
bin/einrichten.sh         # baut, legt nach ~/.local/bin, trägt den Starter ein
bin/app-bauen.sh          # die Android-App; --installieren <seriennr> spielt sie auf
```

**Die Oberfläche steckt im Binary.** Änderungen an `internal/web/dateien/`
wirken erst nach `bin/bau.sh` — sonst sucht man den Fehler im Programm.

Die App braucht Gradle 9.7.1 + AGP 9.3.2 + JDK 21. AGP 9 bringt Kotlin selbst
mit; ein zusätzliches `kotlin("android")` scheitert. Das Compose-Plugin wird
dagegen gebraucht.

## Befehle

```sh
esstracker             Fenster öffnen (Vorgabe)
esstracker dienst      nur ausliefern, ohne Fenster
esstracker heute       den heutigen Stand im Terminal
esstracker koppeln     Kopplungsfenster öffnen und warten
esstracker ausfuhr     alles als CSV auf die Standardausgabe
esstracker einrichten  Starter im Anwendungsmenü anlegen
```

Umgebung: `ESSTRACKER_DATEN`, `ESSTRACKER_ADRESSE`, `ESSTRACKER_EMPFANG`.

## Daten

`~/.local/share/esstracker/tagebuch.json` — eine Datei, atomar geschrieben
(erst daneben, dann umbenannt). Keine Datenbank: ein Eintrag ist knapp ein
Kilobyte, und selbst ein Dutzend am Tag bleibt im Jahr bei wenigen Megabyte.

**Ein Eintrag ist unveränderlich.** Er beschreibt ein Ereignis in der
Vergangenheit. Jeder trägt eine ID, die das Handy vergibt; ein Eintrag, der
schon liegt, wird beim zweiten Mal nicht noch einmal angelegt. Genau das macht
die Warteschlange im Handy gefahrlos: sie schickt nach, bis der Rechner
bestätigt, und muss dabei nicht mitzählen, was angekommen ist.

Gespeichert sind nur die Nährwerte je 100 g, so wie sie in der Datenbank
stehen. Was eine Portion ergibt, wird gerechnet. Sonst gäbe es zwei Zahlen,
die dasselbe behaupten, und irgendwann behaupten sie Verschiedenes.

## Woher die Produkte kommen

[Open Food Facts](https://world.openfoodfacts.org) — offen, rund vier
Millionen Produkte, viele mit deutschem Namen. Zwei Wege:

- Strichcode: `api/v2/product/<code>.json`
- Text: `search.openfoodfacts.org/search` (der Nachfolger des abgeschalteten
  `cgi/search.pl`, das nur noch eine Wartungsseite liefert)

Es ist eine Sammlung von Freiwilligen. Nährwerte fehlen dort öfter, und
gelegentlich stehen Name und Marke vertauscht. Produkte ganz ohne
Energieangabe blendet die Suche aus — als Eintrag wären sie wertlos.

## Die App

`android/` — Kotlin und Compose, dieselben Farbwerte wie die Oberfläche hier.
Drei Reiter: Heute, Eintragen, Rechner.

**Ab Android 16 braucht sie `ACCESS_LOCAL_NETWORK`.** `INTERNET` allein reicht
nicht mehr, um den eigenen Rechner zu erreichen; ohne die Erlaubnis läuft jede
Verbindung dorthin in einen Zeitablauf, ohne dass irgendwo der Grund steht.
Die App fragt beim ersten Griff nach dem Rechner danach und sagt, wozu.

Der Strichcode-Leser bringt sein Modell im APK mit: Scannen geht ohne Netz und
ohne Play-Dienste. Im Supermarkt ist oft beides nicht da.

Was aufs Handy gehört, bleibt eine Woche liegen; was älter ist und beim
Rechner angekommen, wird dort weggeräumt.

## Belege

`belege/` — Bildschirmfotos beider Seiten aus dem Durchstich vom 15.09.2026:
Suche in Open Food Facts, Eintrag im Emulator, derselbe Eintrag auf dem
Rechner.
