package de.esstracker.android

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.UUID

/**
 * Das Tagebuch auf dem Handy.
 *
 * Es ist bewusst kein Zwischenspeicher, sondern die Quelle: der Eintrag
 * entsteht hier, wird hier gezaehlt und geht von hier aus zum Rechner. Wenn
 * der Rechner aus ist, ist das kein Sonderfall — der Eintrag liegt dann eben
 * in der Warteschlange und geht spaeter raus.
 *
 * Gespeichert wird als JSON-Datei, nicht in einer Datenbank. Ein Eintrag ist
 * knapp ein Kilobyte, und was bereits beim Rechner liegt, wird nach einer
 * Woche weggeraeumt — die Datei bleibt damit im Bereich weniger hundert
 * Kilobyte, und Room waere eine Abhaengigkeit und eine Schemawanderung mehr.
 *
 * Geschrieben wird ueber eine Nachbardatei und ein Umbenennen: ein Absturz
 * mitten im Schreiben hinterlaesst so die alte, vollstaendige Datei statt
 * einer halben neuen.
 */
class Tagebuch(private val zusammenhang: Context) {

    private val datei get() = File(zusammenhang.filesDir, "tagebuch.json")

    /** Alles, was auf dem Handy liegt. */
    var eintraege: List<Eintrag> = emptyList()
        private set

    /** Das Tagesziel. Es kommt vom Rechner und wird hier nur gespiegelt. */
    var ziel: Int = 2000
        private set

    init { lies() }

    // ---------- Lesen und Schreiben ----------

    private fun lies() {
        if (!datei.exists()) return
        try {
            val wurzel = JSONObject(datei.readText())
            ziel = wurzel.optInt("ziel", 2000).coerceIn(500, 20000)
            val liste = wurzel.optJSONArray("eintraege") ?: JSONArray()
            eintraege = (0 until liste.length()).mapNotNull { Eintrag.ausJson(liste.getJSONObject(it)) }
        } catch (fehler: Exception) {
            // Eine kaputte Datei darf die App nicht am Starten hindern. Sie
            // wird beiseitegelegt statt geloescht: vielleicht laesst sich von
            // Hand noch etwas retten, und stillschweigend Daten wegzuwerfen
            // waere das Schlimmste, was ein Tagebuch tun kann.
            datei.renameTo(File(zusammenhang.filesDir, "tagebuch.kaputt.json"))
            eintraege = emptyList()
        }
    }

    private fun schreib() {
        val liste = JSONArray()
        eintraege.forEach { liste.put(it.alsJson()) }
        val wurzel = JSONObject().put("ziel", ziel).put("eintraege", liste)

        val neben = File(zusammenhang.filesDir, "tagebuch.neu.json")
        neben.writeText(wurzel.toString())
        neben.renameTo(datei)
    }

    // ---------- Eintragen ----------

    /** Legt einen Eintrag an. Er gilt sofort und geht danach zum Rechner. */
    fun trageEin(eintrag: Eintrag) {
        eintraege = (eintraege + eintrag).sortedBy { it.zeit }
        schreib()
    }

    fun loesche(id: String) {
        eintraege = eintraege.filterNot { it.id == id }
        schreib()
    }

    /**
     * Hakt ab, was beim Rechner angekommen ist.
     *
     * Der Rechner meldet die IDs, die bei ihm liegen — auch die, die er schon
     * kannte. Deshalb genuegt hier ein Abgleich ueber die ID, und das Handy
     * muss nicht mitzaehlen, was es wann geschickt hat.
     */
    fun abgehakt(ids: Collection<String>) {
        if (ids.isEmpty()) return
        val menge = ids.toHashSet()
        eintraege = eintraege.map { if (it.id in menge) it.copy(gesendet = true) else it }
        raeumeAuf()
        schreib()
    }

    fun setzeZiel(kcal: Int) {
        val neu = kcal.coerceIn(500, 20000)
        if (neu == ziel) return
        ziel = neu
        schreib()
    }

    /**
     * Wirft weg, was beim Rechner liegt und aelter als eine Woche ist.
     *
     * Die Woche ist der Puffer: das Handy zeigt die letzten Tage, und wer
     * heute Abend nachsehen will, was er Montag gegessen hat, soll das ohne
     * den Rechner koennen. Was aelter ist, steht dort vollstaendig — und
     * genau dafuer ist der Rechner da.
     */
    private fun raeumeAuf() {
        val grenze = LocalDate.now().minusDays(7).toString()
        eintraege = eintraege.filterNot { it.gesendet && it.tag < grenze }
    }

    // ---------- Fragen ----------

    /** Was noch nicht beim Rechner liegt, aelteste zuerst. */
    fun warteschlange(): List<Eintrag> = eintraege.filterNot { it.gesendet }

    fun vonHeute(): List<Eintrag> = amTag(heute())

    fun amTag(tag: String): List<Eintrag> = eintraege.filter { it.tag == tag }

    /** Die einzige Zahl, die diese App gross anzeigt. */
    fun kcalHeute(): Int = vonHeute().sumOf { it.kcal() }.toInt()

    /** Die letzten Tage mit Eintraegen, neueste zuerst. */
    fun tage(): List<String> = eintraege.map { it.tag }.distinct().sortedDescending()

    companion object {
        fun heute(): String = LocalDate.now().toString()
    }
}

/**
 * Ein Eintrag — dasselbe, was der Rechner als JSON empfaengt.
 *
 * `je100` haelt die Naehrwerte, wie sie in der Datenbank stehen. Was eine
 * Portion ergibt, wird gerechnet und nicht gespeichert: sonst gaebe es zwei
 * Zahlen, die dasselbe behaupten.
 */
data class Eintrag(
    val id: String = UUID.randomUUID().toString(),
    val zeit: String = OffsetDateTime.now().format(DateTimeFormatter.ISO_OFFSET_DATE_TIME),
    val tag: String = LocalDate.now().toString(),
    val mahlzeit: String = mahlzeitJetzt(),
    val name: String,
    val marke: String = "",
    val barcode: String = "",
    val menge: Double,
    val einheit: String = "g",
    val je100: Naehrwerte,
    val quelle: String = "off",
    /** Ob der Rechner ihn bestaetigt hat. Steht nur hier, nicht im JSON. */
    val gesendet: Boolean = false,
) {
    fun kcal(): Double = je100.kcal * menge / 100.0

    /** Die Uhrzeit, in der Zone, in der gegessen wurde. */
    fun uhr(): String = try {
        OffsetDateTime.parse(zeit).format(DateTimeFormatter.ofPattern("HH:mm"))
    } catch (fehler: Exception) {
        ""
    }

    fun alsJson(): JSONObject = JSONObject()
        .put("id", id).put("zeit", zeit).put("tag", tag).put("mahlzeit", mahlzeit)
        .put("name", name).put("marke", marke).put("barcode", barcode)
        .put("menge", menge).put("einheit", einheit)
        .put("je100", je100.alsJson()).put("quelle", quelle)
        .put("gesendet", gesendet)

    companion object {
        fun ausJson(o: JSONObject): Eintrag? {
            val name = o.optString("name")
            if (name.isEmpty()) return null
            return Eintrag(
                id = o.optString("id").ifEmpty { UUID.randomUUID().toString() },
                zeit = o.optString("zeit"),
                tag = o.optString("tag"),
                mahlzeit = o.optString("mahlzeit", "snack"),
                name = name,
                marke = o.optString("marke"),
                barcode = o.optString("barcode"),
                menge = o.optDouble("menge", 0.0),
                einheit = o.optString("einheit", "g"),
                je100 = Naehrwerte.ausJson(o.optJSONObject("je100") ?: JSONObject()),
                quelle = o.optString("quelle", "off"),
                gesendet = o.optBoolean("gesendet", false),
            )
        }
    }
}

/** Naehrwerte je 100 g beziehungsweise 100 ml — so, wie sie in der Datenbank stehen. */
data class Naehrwerte(
    val kcal: Double = 0.0,
    val kj: Double = 0.0,
    val fett: Double = 0.0,
    val gesaettigt: Double = 0.0,
    val kohlenhydrate: Double = 0.0,
    val zucker: Double = 0.0,
    val ballaststoffe: Double = 0.0,
    val eiweiss: Double = 0.0,
    val salz: Double = 0.0,
) {
    fun alsJson(): JSONObject = JSONObject()
        .put("kcal", kcal).put("kj", kj)
        .put("fett", fett).put("gesaettigt", gesaettigt)
        .put("kohlenhydrate", kohlenhydrate).put("zucker", zucker)
        .put("ballaststoffe", ballaststoffe).put("eiweiss", eiweiss).put("salz", salz)

    companion object {
        fun ausJson(o: JSONObject) = Naehrwerte(
            kcal = o.optDouble("kcal", 0.0),
            kj = o.optDouble("kj", 0.0),
            fett = o.optDouble("fett", 0.0),
            gesaettigt = o.optDouble("gesaettigt", 0.0),
            kohlenhydrate = o.optDouble("kohlenhydrate", 0.0),
            zucker = o.optDouble("zucker", 0.0),
            ballaststoffe = o.optDouble("ballaststoffe", 0.0),
            eiweiss = o.optDouble("eiweiss", 0.0),
            salz = o.optDouble("salz", 0.0),
        )
    }
}

/**
 * Raet die Mahlzeit aus der Uhrzeit.
 *
 * Geraten, nicht bestimmt: die Wahl steht im Erfassen-Bildschirm daneben und
 * laesst sich mit einem Griff aendern. Der Vorschlag spart in den meisten
 * Faellen genau diesen Griff.
 */
fun mahlzeitJetzt(): String {
    val stunde = OffsetDateTime.now(ZoneId.systemDefault()).hour
    return when (stunde) {
        in 5..10 -> "fruehstueck"
        in 11..14 -> "mittag"
        in 17..21 -> "abend"
        else -> "snack"
    }
}

val MahlzeitNamen = linkedMapOf(
    "fruehstueck" to "Frühstück",
    "mittag" to "Mittag",
    "abend" to "Abend",
    "snack" to "Zwischendurch",
)

/**
 * Dieselben vier, kurz genug fuer vier Knoepfe nebeneinander.
 *
 * Abgeschnittene Woerter waeren die schlechtere Loesung: „Frühstüc" sieht aus
 * wie ein Fehler, „Früh" wie eine Abkuerzung. Ueber den Listen steht weiter
 * das ganze Wort.
 */
val MahlzeitKurz = linkedMapOf(
    "fruehstueck" to "Früh",
    "mittag" to "Mittag",
    "abend" to "Abend",
    "snack" to "Snack",
)
