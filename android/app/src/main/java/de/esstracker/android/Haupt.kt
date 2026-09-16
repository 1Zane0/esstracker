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
    override fun onCreate(gespeichert: Bundle?) {
        super.onCreate(gespeichert)
        enableEdgeToEdge()
        val zustand = Zustand(Tagebuch(this), Rechner(this), lifecycleScope)
        setContent {
            EsstrackerGestalt {
                Geruest(zustand)
            }
        }
    }
}

/** Welcher Bildschirm gerade offen ist. */
enum class Seite { Heute, Erfassen, Einstellungen }

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
    private val bereich: CoroutineScope,
) {
    var seite by mutableStateOf(Seite.Heute)
    var meldung by mutableStateOf<String?>(null)

    /** Die Eintraege von heute, neu gelesen nach jeder Aenderung. */
    var heute by mutableStateOf(tagebuch.vonHeute())
        private set
    var kcalHeute by mutableStateOf(tagebuch.kcalHeute())
        private set
    var ziel by mutableStateOf(tagebuch.ziel)
        private set

    /** Wie viele Eintraege noch beim Rechner fehlen. */
    var offen by mutableStateOf(tagebuch.warteschlange().size)
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
        heute = tagebuch.vonHeute()
        kcalHeute = tagebuch.kcalHeute()
        ziel = tagebuch.ziel
        offen = tagebuch.warteschlange().size
        gekoppelt = rechner.gekoppelt
        rechnerName = rechner.name
    }

    fun sage(text: String?) { meldung = text }

    // ---------- Eintragen ----------

    fun trageEin(eintrag: Eintrag) {
        tagebuch.trageEin(eintrag)
        frischAuf()
        sage("„${eintrag.name}“ eingetragen · ${eintrag.kcal().toInt()} kcal")
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

    // ---------- Uebertragen ----------

    /** Leert die Warteschlange. `still` unterdrueckt die Erfolgsmeldung. */
    fun uebertrage(still: Boolean = false) {
        if (sendetGerade) return
        val stapel = tagebuch.warteschlange()
        if (stapel.isEmpty()) {
            if (!still) sage("Es liegt nichts zum Übertragen bereit.")
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
        }
    }

    // ---------- Suchen ----------

    fun sucheProdukt() {
        val wort = suchtext.trim()
        if (wort.length < 2) return
        suchtGerade = true
        ohneTreffer = false
        treffer.clear()
        bereich.launch {
            val gefunden = withContext(Dispatchers.IO) { Datenbank.ueberText(wort) }
            treffer.addAll(gefunden)
            suchtGerade = false
            ohneTreffer = gefunden.isEmpty()
            if (gefunden.isEmpty()) {
                sage("Nichts gefunden — du kannst es selbst eintragen.")
            }
        }
    }

    fun sucheBarcode(code: String) {
        suchtGerade = true
        bereich.launch {
            val gefunden = withContext(Dispatchers.IO) { Datenbank.ueberBarcode(code) }
            suchtGerade = false
            when {
                gefunden == null -> {
                    ohneTreffer = true
                    sage("$code steht nicht in der Datenbank.")
                }
                !gefunden.hatEnergie -> {
                    ohneTreffer = true
                    sage("„${gefunden.name}“ hat dort keine Nährwerte hinterlegt.")
                }
                else -> gewaehlt = gefunden
            }
        }
    }

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

    fun trenne() {
        rechner.vergiss()
        frischAuf()
        sage("Rechner getrennt. Die Einträge bleiben hier liegen.")
    }
}
