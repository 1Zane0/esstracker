package de.esstracker.android

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.util.UUID
import kotlin.math.abs
import kotlin.math.roundToLong

/**
 * Die Einkaufsliste auf dem Handy.
 *
 * Sie ist — anders als das Tagebuch — nicht die Quelle, sondern eine
 * Abschrift: geschrieben wird sie am Rechner, wo Claude das Tagebuch kennt.
 * Hier wird sie abgearbeitet, und genau dafuer liegt sie hier: im Laden ist
 * kein Rechner.
 *
 * Damit das auch ohne Netz geht, merkt sich diese Klasse jede Aenderung, die
 * hier entsteht — Haken, verstellte Mengen, dazugeschriebene und
 * weggenommene Posten. Beim naechsten Abgleich gehen sie raus und werden
 * bestaetigt; bis dahin liegen sie auf der Abschrift obenauf. Ohne das kaeme
 * beim naechsten Abgleich die Liste vom Rechner zurueck und machte im Laden
 * drei Haken wieder rueckgaengig.
 *
 * Jede Aenderung traegt ihre Zeit. Am Rechner gewinnt die juengere, nicht die
 * zuletzt angekommene.
 */
class Einkaufsliste(private val zusammenhang: Context) {

    private val datei get() = File(zusammenhang.filesDir, "einkauf.json")

    /** Die Liste, wie sie nach den eigenen Aenderungen aussieht. */
    var liste: List<EinkaufsPosten> = emptyList()
        private set

    /** Wann der Rechner die Liste zuletzt geaendert hat. */
    var stand: String = ""
        private set

    /**
     * Die Einheiten und Abteilungen kommen vom Rechner. Bis zum ersten
     * Abgleich steht hier, was die App selbst mitbringt — sonst hat der erste
     * Posten, den jemand hier antippt, keine Einheit zur Auswahl.
     */
    var einheiten: List<Mengeneinheit> = Mengeneinheit.Vorrat
        private set
    var gruppen: List<String> = VorratGruppen
        private set

    /** Was noch nicht beim Rechner ist. */
    private var aenderungen: MutableMap<String, Aenderung> = mutableMapOf()
    private var neue: MutableList<EinkaufsPosten> = mutableListOf()
    private var weg: MutableSet<String> = mutableSetOf()

    val offeneAenderungen: Int get() = aenderungen.size + neue.size + weg.size

    init { lies() }

    // ---------- Lesen und Schreiben ----------

    private fun lies() {
        if (!datei.exists()) return
        try {
            val wurzel = JSONObject(datei.readText())
            stand = wurzel.optString("stand")
            liste = wurzel.optJSONArray("posten").mapJson { EinkaufsPosten.ausJson(it) }
            wurzel.optJSONArray("einheiten")?.let { roh ->
                val gelesen = roh.mapJson { Mengeneinheit.ausJson(it) }
                if (gelesen.isNotEmpty()) einheiten = gelesen
            }
            wurzel.optJSONArray("gruppen")?.let { roh ->
                val gelesen = (0 until roh.length()).map { roh.getString(it) }
                if (gelesen.isNotEmpty()) gruppen = gelesen
            }
            aenderungen = wurzel.optJSONArray("aenderungen")
                .mapJson { Aenderung.ausJson(it) }
                .associateBy { it.id }
                .toMutableMap()
            neue = wurzel.optJSONArray("neue").mapJson { EinkaufsPosten.ausJson(it) }.toMutableList()
            weg = wurzel.optJSONArray("weg").let { roh ->
                if (roh == null) mutableSetOf() else (0 until roh.length()).map { roh.getString(it) }.toMutableSet()
            }
        } catch (fehler: Exception) {
            // Anders als beim Tagebuch geht hier nichts verloren, was es nur
            // hier gibt: die Liste kommt beim naechsten Abgleich wieder.
            liste = emptyList()
        }
    }

    private fun schreib() {
        val wurzel = JSONObject()
            .put("stand", stand)
            .put("posten", JSONArray().apply { liste.forEach { put(it.alsJson()) } })
            .put("einheiten", JSONArray().apply { einheiten.forEach { put(it.alsJson()) } })
            .put("gruppen", JSONArray().apply { gruppen.forEach { put(it) } })
            .put("aenderungen", JSONArray().apply { aenderungen.values.forEach { put(it.alsJson()) } })
            .put("neue", JSONArray().apply { neue.forEach { put(it.alsJson()) } })
            .put("weg", JSONArray().apply { weg.forEach { put(it) } })

        val neben = File(zusammenhang.filesDir, "einkauf.neu.json")
        neben.writeText(wurzel.toString())
        neben.renameTo(datei)
    }

    // ---------- Vom Rechner ----------

    /**
     * Uebernimmt den Stand vom Rechner und legt die eigenen, noch nicht
     * bestaetigten Aenderungen darueber.
     */
    fun uebernimm(vomRechner: RechnerListe) {
        stand = vomRechner.stand
        if (vomRechner.einheiten.isNotEmpty()) einheiten = vomRechner.einheiten
        if (vomRechner.gruppen.isNotEmpty()) gruppen = vomRechner.gruppen

        val fertig = vomRechner.posten
            .filterNot { it.id in weg }
            .map { p -> aenderungen[p.id]?.auf(p) ?: p }
            .toMutableList()
        // Was hier entstanden und noch nicht bestaetigt ist, gehoert dazu —
        // sonst verschwindet der eben eingetippte Posten, bis er durchkommt.
        val da = fertig.map { it.id }.toHashSet()
        fertig.addAll(neue.filterNot { it.id in da })

        liste = sortiere(fertig)
        schreib()
    }

    /**
     * Hakt ab, was der Rechner bestaetigt hat — und nur das.
     *
     * Waehrend die Anfrage lief, kann im Laden schon der naechste Haken
     * gesetzt worden sein. Alles zu leeren hiesse, genau den zu verlieren.
     */
    fun bestaetigt(gesendetAenderungen: List<Aenderung>, gesendetNeue: List<EinkaufsPosten>, gesendetWeg: List<String>) {
        for (a in gesendetAenderungen) {
            if (aenderungen[a.id]?.geaendert == a.geaendert) aenderungen.remove(a.id)
        }
        val ids = gesendetNeue.map { it.id }.toHashSet()
        neue.removeAll { it.id in ids }
        weg.removeAll(gesendetWeg.toSet())
        schreib()
    }

    /** Was beim naechsten Abgleich rausgeht. */
    fun offeneListe(): Triple<List<Aenderung>, List<EinkaufsPosten>, List<String>> =
        Triple(aenderungen.values.toList(), neue.toList(), weg.toList())

    // ---------- Hier geaendert ----------

    fun hake(posten: EinkaufsPosten, erledigt: Boolean) {
        merke(posten.copy(erledigt = erledigt, geaendert = jetzt()))
    }

    /**
     * Ein Druck auf + oder −. Um wie viel es geht, sagt die Einheit — dieselbe
     * Schrittweite wie am Rechner, weil die Liste von dort kommt.
     */
    fun schritt(posten: EinkaufsPosten, richtung: Int) {
        val e = einheitZu(posten.einheit)
        var menge = posten.menge + e.schritt * (if (richtung < 0) -1 else 1)
        menge = (menge * 1000).roundToLong() / 1000.0
        if (menge < e.schritt) menge = e.schritt
        if (menge > HOECHSTE_MENGE) menge = HOECHSTE_MENGE
        if (menge == posten.menge) return
        merke(posten.copy(menge = menge, geaendert = jetzt()))
    }

    /** Schreibt einen Posten dazu — was im Laden einfällt. */
    fun legeAn(name: String, menge: Double, einheit: String, gruppe: String): EinkaufsPosten? {
        val sauber = name.trim()
        if (sauber.isEmpty()) return null
        if (liste.any { it.name.equals(sauber, ignoreCase = true) }) return null
        val e = einheitZu(einheit)
        val posten = EinkaufsPosten(
            id = UUID.randomUUID().toString(),
            name = sauber,
            menge = if (menge > 0) menge else e.schritt,
            einheit = e.name,
            gruppe = gruppe.ifEmpty { GRUPPE_SONSTIGES },
            grund = "",
            von = "hand",
            angelegt = jetzt(),
        )
        neue.add(posten)
        liste = sortiere(liste + posten)
        schreib()
        return posten
    }

    fun nimmWeg(posten: EinkaufsPosten) {
        liste = liste.filterNot { it.id == posten.id }
        aenderungen.remove(posten.id)
        // Was hier entstand und nie beim Rechner war, muss dort auch nicht
        // weggenommen werden.
        if (neue.removeAll { it.id == posten.id }) {
            schreib()
            return
        }
        weg.add(posten.id)
        schreib()
    }

    /** Legt eine Aenderung auf die Abschrift und in die Warteschlange. */
    private fun merke(neuerStand: EinkaufsPosten) {
        liste = liste.map { if (it.id == neuerStand.id) neuerStand else it }
        val fuerRechner = neue.indexOfFirst { it.id == neuerStand.id }
        if (fuerRechner >= 0) {
            // Ein Posten, den der Rechner noch gar nicht kennt: dann aendert
            // sich das, was rausgeht, und nicht etwas dort.
            neue[fuerRechner] = neuerStand
        } else {
            aenderungen[neuerStand.id] = Aenderung(
                id = neuerStand.id, erledigt = neuerStand.erledigt,
                menge = neuerStand.menge, einheit = neuerStand.einheit,
                geaendert = neuerStand.geaendert.ifEmpty { jetzt() },
            )
        }
        schreib()
    }

    // ---------- Fragen ----------

    fun offen(): Int = liste.count { !it.erledigt }

    fun einheitZu(name: String): Mengeneinheit =
        einheiten.firstOrNull { it.name.equals(name, ignoreCase = true) }
            ?: einheiten.firstOrNull { it.eins.equals(name, ignoreCase = true) }
            ?: einheiten.firstOrNull()
            ?: Mengeneinheit.Vorrat.first()

    /** Die Menge, wie sie auf einem Zettel stuende: "2 Packungen", "500 g". */
    fun mengeText(posten: EinkaufsPosten): String {
        val e = einheitZu(posten.einheit)
        val name = if (abs(posten.menge - 1) < 0.001) e.eins else e.viele
        val zahl = if (posten.menge == posten.menge.toLong().toDouble()) {
            posten.menge.toLong().toString()
        } else {
            posten.menge.toString().replace('.', ',')
        }
        return "$zahl $name"
    }

    /** Nach Abteilung, in der Reihenfolge, in der man den Laden durchgeht. */
    private fun sortiere(roh: List<EinkaufsPosten>): List<EinkaufsPosten> {
        val platz = gruppen.withIndex().associate { (i, g) -> g to i }
        return roh.sortedBy { platz[it.gruppe] ?: gruppen.size }
    }

    companion object {
        const val HOECHSTE_MENGE = 10000.0
        const val GRUPPE_SONSTIGES = "Sonstiges"

        /** Die Abteilungen, bis der erste Abgleich die echten bringt. */
        val VorratGruppen = listOf(
            "Obst & Gemüse", "Brot & Backwaren", "Kühlregal", "Fleisch & Fisch",
            "Trockenwaren", "Konserven & Gläser", "Tiefkühl", "Getränke",
            "Süßes & Snacks", "Haushalt", "Sonstiges",
        )

        fun jetzt(): String = OffsetDateTime.now().format(DateTimeFormatter.ISO_OFFSET_DATE_TIME)
    }
}

/** Eine Zeile auf der Liste. Dasselbe, was der Rechner schickt. */
data class EinkaufsPosten(
    val id: String,
    val name: String,
    val menge: Double,
    val einheit: String,
    val gruppe: String,
    val grund: String = "",
    val von: String = "ki",
    val erledigt: Boolean = false,
    val geaendert: String = "",
    val angelegt: String = "",
) {
    fun alsJson(): JSONObject = JSONObject()
        .put("id", id).put("name", name).put("menge", menge).put("einheit", einheit)
        .put("gruppe", gruppe).put("grund", grund).put("von", von)
        .put("erledigt", erledigt).put("geaendert", geaendert).put("angelegt", angelegt)

    companion object {
        fun ausJson(o: JSONObject): EinkaufsPosten? {
            val name = o.optString("name").trim()
            val id = o.optString("id")
            if (name.isEmpty() || id.isEmpty()) return null
            return EinkaufsPosten(
                id = id,
                name = name,
                menge = o.optDouble("menge", 1.0),
                einheit = o.optString("einheit", "stueck"),
                gruppe = o.optString("gruppe", Einkaufsliste.GRUPPE_SONSTIGES),
                grund = o.optString("grund"),
                von = o.optString("von", "ki"),
                erledigt = o.optBoolean("erledigt", false),
                geaendert = o.optString("geaendert"),
                angelegt = o.optString("angelegt"),
            )
        }
    }
}

/** Eine Mengeneinheit samt Schrittweite — die Vorgabe kommt vom Rechner. */
data class Mengeneinheit(
    val name: String,
    val eins: String,
    val viele: String,
    val schritt: Double,
) {
    fun alsJson(): JSONObject = JSONObject()
        .put("name", name).put("eins", eins).put("viele", viele).put("schritt", schritt)

    companion object {
        fun ausJson(o: JSONObject): Mengeneinheit? {
            val name = o.optString("name")
            if (name.isEmpty()) return null
            return Mengeneinheit(
                name = name,
                eins = o.optString("eins", name),
                viele = o.optString("viele", o.optString("eins", name)),
                schritt = o.optDouble("schritt", 1.0).takeIf { it > 0 } ?: 1.0,
            )
        }

        /** Was die App mitbringt, bis der erste Abgleich die echten liefert. */
        val Vorrat = listOf(
            Mengeneinheit("stueck", "Stück", "Stück", 1.0),
            Mengeneinheit("g", "g", "g", 50.0),
            Mengeneinheit("kg", "kg", "kg", 0.5),
            Mengeneinheit("ml", "ml", "ml", 100.0),
            Mengeneinheit("l", "l", "l", 0.5),
            Mengeneinheit("packung", "Packung", "Packungen", 1.0),
            Mengeneinheit("beutel", "Beutel", "Beutel", 1.0),
            Mengeneinheit("dose", "Dose", "Dosen", 1.0),
            Mengeneinheit("glas", "Glas", "Gläser", 1.0),
            Mengeneinheit("becher", "Becher", "Becher", 1.0),
            Mengeneinheit("flasche", "Flasche", "Flaschen", 1.0),
            Mengeneinheit("bund", "Bund", "Bund", 1.0),
        )
    }
}

/** Eine Aenderung, die noch beim Rechner fehlt. */
data class Aenderung(
    val id: String,
    val erledigt: Boolean,
    val menge: Double,
    val einheit: String,
    val geaendert: String,
) {
    /** Legt die Aenderung auf den Stand vom Rechner. */
    fun auf(p: EinkaufsPosten): EinkaufsPosten =
        p.copy(erledigt = erledigt, menge = menge, einheit = einheit, geaendert = geaendert)

    fun alsJson(): JSONObject = JSONObject()
        .put("id", id).put("erledigt", erledigt).put("menge", menge)
        .put("einheit", einheit).put("geaendert", geaendert)

    companion object {
        fun ausJson(o: JSONObject): Aenderung? {
            val id = o.optString("id")
            if (id.isEmpty()) return null
            return Aenderung(
                id = id,
                erledigt = o.optBoolean("erledigt", false),
                menge = o.optDouble("menge", 0.0),
                einheit = o.optString("einheit"),
                geaendert = o.optString("geaendert"),
            )
        }
    }
}

/** Was der Rechner beim Abgleich zur Liste schickt. */
data class RechnerListe(
    val posten: List<EinkaufsPosten>,
    val stand: String,
    val einheiten: List<Mengeneinheit>,
    val gruppen: List<String>,
)

/** Liest eine Liste von Objekten, ohne bei einem kaputten Eintrag aufzugeben. */
private fun <T> JSONArray?.mapJson(wandle: (JSONObject) -> T?): List<T> {
    if (this == null) return emptyList()
    return (0 until length()).mapNotNull { i ->
        val o = optJSONObject(i) ?: return@mapNotNull null
        wandle(o)
    }
}
