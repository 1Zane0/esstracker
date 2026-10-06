package de.esstracker.android

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Esstracker fuers Handy.
 *
 * Ein Tracker, eine Zahl: die Kalorien von heute. Alles andere — Marke,
 * Strichcode, Fett, Zucker, Salz, die Uhrzeit — wird zwar erfasst und
 * uebertragen, aber nicht hier angezeigt. Das ist der Entwurf und nicht
 * Sparsamkeit: wer beim Eintragen eine Naehrwerttabelle liest, traegt nicht
 * mehr ein. Die vollstaendige Ansicht steht auf dem Rechner.
 */
class Haupt : ComponentActivity() {
    private lateinit var zustand: Zustand

    override fun onCreate(gespeichert: Bundle?) {
        super.onCreate(gespeichert)
        enableEdgeToEdge()
        zustand = Zustand(Tagebuch(this), Rechner(this), EigeneDatenbank(this), Einkaufsliste(this), lifecycleScope)
        setContent {
            EsstrackerGestalt {
                Geruest(zustand)
            }
        }
    }

    /**
     * Zurueck aus dem Hintergrund: nachschicken und abgleichen. Die App liegt
     * meist tagelang offen im Hintergrund, und ohne das kaeme ein geschaetzter
     * Doener erst beim naechsten Eintrag an.
     */
    override fun onResume() {
        super.onResume()
        zustand.uebertrage(still = true)
    }
}

/** Welcher Bildschirm gerade offen ist. */
enum class Seite { Heute, Erfassen, Einkauf, Einstellungen }

/**
 * Der Zustand der App.
 *
 * Alles Veraenderliche steht hier an einer Stelle, damit die Bildschirme
 * reines Zeichnen bleiben. Netzarbeit laeuft grundsaetzlich ueber
 * `Dispatchers.IO` — eine Abfrage an Open Food Facts ueber Mobilfunk dauert
 * gut eine Sekunde, und die gehoert nicht auf den Zeichen-Thread.
 */
class Zustand(
    val tagebuch: Tagebuch,
    val rechner: Rechner,
    val eigene: EigeneDatenbank,
    val einkauf: Einkaufsliste,
    private val bereich: CoroutineScope,
) {
    var seite by mutableStateOf(Seite.Heute)
    var meldung by mutableStateOf<String?>(null)

    /** Die Eintraege von heute, neu gelesen nach jeder Aenderung. */
    var heute by mutableStateOf(tagebuch.vonHeute())
        private set
    var kcalHeute by mutableStateOf(tagebuch.kcalHeute())
        private set
    /** Wie viele Eintraege von heute noch ohne Naehrwerte zaehlen. */
    var offenHeute by mutableStateOf(tagebuch.offenHeute())
        private set
    var ziel by mutableStateOf(tagebuch.ziel)
        private set

    /** Wie viele Eintraege noch beim Rechner fehlen. */
    var offen by mutableStateOf(tagebuch.warteschlange().size)
        private set

    /**
     * Die Einkaufsliste, gespiegelt fuer Compose.
     *
     * Aus demselben Grund wie `gekoppelt`: die Liste liegt in einer Datei, und
     * eine Datei aendert sich fuer Compose unsichtbar. Ohne das bliebe der
     * Haken aus, den man gerade gesetzt hat.
     */
    var einkaufsListe by mutableStateOf(einkauf.liste)
        private set
    var einkaufOffen by mutableStateOf(einkauf.offen())
        private set
    /** Wie viele Aenderungen an der Liste noch beim Rechner fehlen. */
    var einkaufWartet by mutableStateOf(einkauf.offeneAenderungen)
        private set
    var gleichtEinkaufAb by mutableStateOf(false)
        private set
    var sendetGerade by mutableStateOf(false)
        private set

    /**
     * Ob ein Rechner gekoppelt ist — als Zustand, nicht als Abfrage.
     *
     * Der Rechner selbst haelt das in den Einstellungen, und die sind fuer
     * Compose unsichtbar: nach dem Koppeln aendert sich dort etwas, ohne dass
     * irgendein Zustand anspringt, und der Bildschirm bleibt auf „koppeln"
     * stehen, obwohl es laengst geklappt hat. Deshalb wird es hier
     * gespiegelt.
     */
    var gekoppelt by mutableStateOf(rechner.gekoppelt)
        private set
    var rechnerName by mutableStateOf(rechner.name)
        private set

    // ---------- Erfassen ----------

    var suchtext by mutableStateOf("")
    var treffer = mutableStateListOf<Datenbank.Treffer>()
    var suchtGerade by mutableStateOf(false)
    var gewaehlt by mutableStateOf<Datenbank.Treffer?>(null)

    /** Ob gerade die Maske zum Selbst-Eintragen offen ist. */
    var selbst by mutableStateOf(false)

    /**
     * Was die Maske zum Selbst-Eintragen schon vorausfuellt: der Suchbegriff,
     * der nichts fand, oder das gescannte Produkt ohne Naehrwerte samt
     * Strichcode. Den Namen hat man dann ja schon einmal getippt.
     */
    var vorlage by mutableStateOf<Datenbank.Treffer?>(null)

    /**
     * Ob eine Suche gelaufen ist und nichts gebracht hat.
     *
     * Genau dann ist der Hinweis aufs Selbst-Eintragen am Platz — davor waere
     * er ein Angebot auf einen Fall, der noch gar nicht eingetreten ist.
     */
    var ohneTreffer by mutableStateOf(false)
        private set

    // ---------- Kopplung ----------

    var funde = mutableStateListOf<Rechner.Fund>()
    var suchtRechner by mutableStateOf(false)

    fun frischAuf() {
        einkaufsListe = einkauf.liste
        einkaufOffen = einkauf.offen()
        einkaufWartet = einkauf.offeneAenderungen
        heute = tagebuch.vonHeute()
        kcalHeute = tagebuch.kcalHeute()
        offenHeute = tagebuch.offenHeute()
        ziel = tagebuch.ziel
        offen = tagebuch.warteschlange().size
        gekoppelt = rechner.gekoppelt
        rechnerName = rechner.name
    }

    fun sage(text: String?) { meldung = text }

    // ---------- Eintragen ----------

    fun trageEin(eintrag: Eintrag) {
        tagebuch.trageEin(eintrag)
        vorlage = null
        frischAuf()
        sage(
            if (eintrag.offen) "„${eintrag.name}“ eingetragen — die Nährwerte schätzt Claude am Rechner."
            else "„${eintrag.name}“ eingetragen · ${eintrag.kcal().toInt()} kcal"
        )
        // Sofort versuchen zu senden. Klappt es nicht, bleibt der Eintrag in
        // der Warteschlange — deshalb gibt es hier auch keine Fehlermeldung:
        // „noch nicht uebertragen“ ist ein Zustand, kein Fehler.
        uebertrage(still = true)
    }

    fun loesche(eintrag: Eintrag) {
        tagebuch.loesche(eintrag.id)
        frischAuf()
        // Dass der Rechner den Eintrag behaelt, ist hier wichtig genug fuer
        // einen eigenen Satz: sonst sucht jemand die geloeschte Zeile dort
        // vergeblich und haelt die Uebertragung fuer kaputt.
        val schon = eintrag.gesendet
        sage(if (schon) "Hier gelöscht. Auf dem Rechner bleibt der Eintrag stehen." else "Gelöscht.")
    }

    // ---------- Einkaufsliste ----------

    /**
     * Abhaken, Menge verstellen, dazuschreiben, wegnehmen.
     *
     * Alles gilt sofort und geht danach raus. Klappt das nicht — im Laden ist
     * der Rechner nicht erreichbar —, bleibt die Aenderung liegen und geht
     * beim naechsten Abgleich mit. „Noch nicht uebertragen" ist hier ein
     * Zustand und kein Fehler, genau wie bei den Eintraegen.
     */
    fun hakeAb(posten: EinkaufsPosten) {
        einkauf.hake(posten, !posten.erledigt)
        frischAuf()
        einkaufSchicken()
    }

    fun postenSchritt(posten: EinkaufsPosten, richtung: Int) {
        einkauf.schritt(posten, richtung)
        frischAuf()
        einkaufSchicken()
    }

    fun postenAnlegen(name: String, menge: Double, einheit: String) {
        val posten = einkauf.legeAn(name, menge, einheit, Einkaufsliste.GRUPPE_SONSTIGES)
        frischAuf()
        if (posten == null) {
            sage("„${name.trim()}“ steht schon auf der Liste.")
            return
        }
        sage("„${posten.name}“ dazugeschrieben.")
        einkaufSchicken()
    }

    fun postenWeg(posten: EinkaufsPosten) {
        einkauf.nimmWeg(posten)
        frischAuf()
        sage("„${posten.name}“ von der Liste genommen.")
        einkaufSchicken()
    }

    /** Der Knopf auf dem Bildschirm: schicken, was liegt, und die Liste holen. */
    fun einkaufAbgleichen() {
        if (!gekoppelt) {
            sage("Noch kein Rechner gekoppelt — siehe Einstellungen.")
            return
        }
        einkaufSchicken(still = false, auchOhneAenderung = true)
    }

    /**
     * Schickt die offenen Aenderungen und uebernimmt die Liste, die
     * zurueckkommt.
     *
     * Bestaetigt wird nur, was wirklich rausging: waehrend die Anfrage lief,
     * kann im Laden schon der naechste Haken gesetzt worden sein.
     */
    fun einkaufSchicken(still: Boolean = true, auchOhneAenderung: Boolean = false) {
        if (!gekoppelt || gleichtEinkaufAb) return
        val (aenderungen, neue, weg) = einkauf.offeneListe()
        if (aenderungen.isEmpty() && neue.isEmpty() && weg.isEmpty() && !auchOhneAenderung) return

        gleichtEinkaufAb = true
        bereich.launch {
            val liste = withContext(Dispatchers.IO) { rechner.sendeEinkauf(aenderungen, neue, weg) }
            gleichtEinkaufAb = false
            if (liste == null) {
                if (!still) sage("Der Rechner ist nicht erreichbar. Die Änderungen gehen später raus.")
                return@launch
            }
            einkauf.bestaetigt(aenderungen, neue, weg)
            einkauf.uebernimm(liste)
            frischAuf()
            if (!still) sage("Liste abgeglichen — ${einkaufOffen} noch zu holen.")
        }
    }

    // ---------- Uebertragen ----------

    /**
     * Leert die Warteschlange und gleicht danach ab. `still` unterdrueckt die
     * Erfolgsmeldung.
     */
    fun uebertrage(still: Boolean = false) {
        if (sendetGerade) return
        val stapel = tagebuch.warteschlange()
        if (stapel.isEmpty()) {
            if (!still) sage("Es liegt nichts zum Übertragen bereit.")
            gleicheAb()
            return
        }
        if (!gekoppelt) {
            if (!still) sage("Noch kein Rechner gekoppelt — siehe Einstellungen.")
            return
        }
        sendetGerade = true
        bereich.launch {
            val abrechnung = withContext(Dispatchers.IO) { rechner.sende(stapel) }
            tagebuch.abgehakt(abrechnung.angenommen)
            abrechnung.ziel?.let { tagebuch.setzeZiel(it) }
            frischAuf()
            sendetGerade = false
            when {
                abrechnung.grund != null -> if (!still) sage(abrechnung.grund)
                !still -> sage(
                    if (abrechnung.neu > 0) "${abrechnung.neu} übertragen."
                    else "Der Rechner hatte schon alles."
                )
            }
            if (!abrechnung.geklappt) return@launch
            gleicheAb()
            // Ging etwas ohne Naehrwerte raus, schaetzt der Rechner jetzt.
            // Das dauert mit Haiku ein paar Sekunden; danach noch einmal
            // nachsehen, damit die Zahl von heute stimmt, solange die App
            // noch offen ist.
            if (stapel.any { it.offen && it.id in abrechnung.angenommen }) {
                delay(ABGLEICH_NACH_SCHAETZUNG_MS)
                gleicheAb(sofort = true)
            }
        }
    }

    private var zuletztAbgeglichen = 0L
    private var gleichtAb = false

    /**
     * Holt eigene Datenbank und Nachtraege vom Rechner.
     *
     * Hoechstens alle halbe Minute, ausser `sofort`: angestossen wird das beim
     * Aufschlagen, nach jeder Uebertragung und aus dem Hintergrund zurueck —
     * und unterwegs laeuft jeder Versuch erst in den Zeitablauf.
     */
    fun gleicheAb(sofort: Boolean = false) {
        if (!gekoppelt || gleichtAb) return
        val jetzt = System.currentTimeMillis()
        if (!sofort && jetzt - zuletztAbgeglichen < ABGLEICH_ABSTAND_MS) return
        zuletztAbgeglichen = jetzt
        gleichtAb = true
        bereich.launch {
            val abgleich = withContext(Dispatchers.IO) { rechner.abgleich() }
            gleichtAb = false
            if (abgleich == null) return@launch
            eigene.ersetze(abgleich.eigene)
            // Die Einkaufsliste kommt beim selben Abgleich mit. Liegt hier
            // noch etwas, das der Rechner nicht kennt, geht das zuerst raus —
            // sonst kaeme seine Fassung zurueck und machte drei Haken aus dem
            // Laden wieder rueckgaengig.
            abgleich.einkauf?.let { vomRechner ->
                einkauf.uebernimm(vomRechner)
                einkaufSchicken()
            }
            val geaendert = tagebuch.uebernimm(abgleich.nachtraege)
            abgleich.ziel?.let { tagebuch.setzeZiel(it) }
            frischAuf()
            if (geaendert > 0) {
                sage(if (geaendert == 1) "1 Eintrag hat jetzt Nährwerte vom Rechner." else "$geaendert Einträge haben jetzt Nährwerte vom Rechner.")
            }
        }
    }

    // ---------- Suchen ----------

    /**
     * Sucht erst in der eigenen Datenbank, dann in Open Food Facts. Die
     * eigenen Treffer stehen sofort da und oben: sie sind das, was man schon
     * einmal gegessen hat, und sie kommen auch ohne Netz.
     */
    fun sucheProdukt() {
        val wort = suchtext.trim()
        if (wort.length < 2) return
        suchtGerade = true
        ohneTreffer = false
        vorlage = null
        treffer.clear()
        treffer.addAll(eigene.suche(wort))
        bereich.launch {
            val gefunden = withContext(Dispatchers.IO) { Datenbank.ueberText(wort) }
            treffer.addAll(gefunden)
            suchtGerade = false
            ohneTreffer = treffer.isEmpty()
            if (treffer.isEmpty()) {
                vorlage = leereVorlage(name = wort)
                sage("Nichts gefunden — Name und Menge genügen.")
            }
        }
    }

    fun sucheBarcode(code: String) {
        // Was schon einmal geschaetzt wurde, kennt die eigene Datenbank —
        // ohne Netz und ohne auf Open Food Facts zu warten.
        eigene.zuBarcode(code)?.let {
            gewaehlt = it
            return
        }
        suchtGerade = true
        bereich.launch {
            val gefunden = withContext(Dispatchers.IO) { Datenbank.ueberBarcode(code) }
            suchtGerade = false
            when {
                gefunden == null -> {
                    ohneTreffer = true
                    vorlage = leereVorlage(barcode = code)
                    sage("$code steht nicht in der Datenbank.")
                }
                !gefunden.hatEnergie -> {
                    ohneTreffer = true
                    vorlage = gefunden
                    sage("„${gefunden.name}“ hat dort keine Nährwerte — Name und Menge genügen.")
                }
                else -> gewaehlt = gefunden
            }
        }
    }

    private fun leereVorlage(name: String = "", barcode: String = "") = Datenbank.Treffer(
        barcode = barcode, name = name, marke = "", menge = "",
        je100 = Naehrwerte(), einheit = "g", quelle = "offen",
    )

    // ---------- Rechner suchen ----------

    fun sucheRechner() {
        suchtRechner = true
        funde.clear()
        bereich.launch {
            val gefunden = withContext(Dispatchers.IO) { rechner.suche() }
            funde.addAll(gefunden)
            suchtRechner = false
            if (gefunden.isEmpty()) {
                sage("Kein Rechner gefunden. Läuft der Esstracker dort, und hängt das Handy im selben WLAN?")
            }
        }
    }

    fun kopple(adresse: String) {
        bereich.launch {
            when (val ergebnis = withContext(Dispatchers.IO) { rechner.kopple(adresse) }) {
                is Rechner.Ergebnis.Fertig -> {
                    tagebuch.setzeZiel(ergebnis.ziel)
                    frischAuf()
                    sage("Mit „${ergebnis.rechner}“ gekoppelt.")
                    funde.clear()
                    uebertrage(still = true)
                }
                is Rechner.Ergebnis.Schiefgelaufen -> sage(ergebnis.grund)
            }
        }
    }

    fun frageAdresse(adresse: String) {
        suchtRechner = true
        bereich.launch {
            val fund = withContext(Dispatchers.IO) { rechner.frage(adresse) }
            suchtRechner = false
            if (fund == null) {
                sage("Unter $adresse antwortet kein Esstracker.")
            } else {
                funde.clear()
                funde.add(fund)
            }
        }
    }

    companion object {
        const val ABGLEICH_ABSTAND_MS = 30_000L
        const val ABGLEICH_NACH_SCHAETZUNG_MS = 25_000L
    }

    fun trenne() {
        rechner.vergiss()
        frischAuf()
        sage("Rechner getrennt. Die Einträge bleiben hier liegen.")
    }
}
