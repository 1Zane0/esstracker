package de.esstracker.android

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.util.Locale

/**
 * Die eigene Datenbank — was Claude am Rechner einmal geschaetzt hat.
 *
 * Wer „Döner mit Soße“ ohne Naehrwerte eintraegt, bekommt die Werte am Rechner
 * geschaetzt, und dort landen sie in einer eigenen Liste. Hier liegt eine
 * Abschrift davon: die Suche findet sie neben Open Food Facts, und ein
 * Strichcode, den die Sammlung nicht kennt, wird beim zweiten Scannen erkannt.
 *
 * Abschrift und nicht Abfrage, weil gesucht wird, wo der Rechner nicht ist —
 * im Supermarkt, beim Imbiss. Beim Abgleich wird sie als Ganzes ersetzt: was am
 * Rechner geloescht wurde, verschwindet damit auch hier, ohne dass jemand
 * Loeschungen mitzaehlen muss.
 */
class EigeneDatenbank(private val zusammenhang: Context) {

    private val datei get() = File(zusammenhang.filesDir, "eigene.json")

    var liste: List<Datenbank.Treffer> = emptyList()
        private set

    init { lies() }

    private fun lies() {
        if (!datei.exists()) return
        liste = try {
            val roh = JSONArray(datei.readText())
            (0 until roh.length()).mapNotNull { ausJson(roh.getJSONObject(it)) }
        } catch (fehler: Exception) {
            // Anders als beim Tagebuch geht hier nichts verloren: die Liste
            // kommt beim naechsten Abgleich vollstaendig wieder.
            emptyList()
        }
    }

    /** Ersetzt die Abschrift durch den Stand vom Rechner. */
    fun ersetze(neu: List<Datenbank.Treffer>) {
        if (neu == liste) return
        liste = neu
        val roh = JSONArray()
        neu.forEach {
            roh.put(
                JSONObject()
                    .put("name", it.name).put("marke", it.marke).put("barcode", it.barcode)
                    .put("einheit", it.einheit).put("je100", it.je100.alsJson())
            )
        }
        val neben = File(zusammenhang.filesDir, "eigene.neu.json")
        neben.writeText(roh.toString())
        neben.renameTo(datei)
    }

    /**
     * Findet, worin jedes Suchwort vorkommt — im Namen oder in der Marke.
     * „heidelbeer“ findet so „Kulturheidelbeeren“, und „döner imbiss“ den
     * Döner vom Imbiss, aber nicht den selbst gemachten.
     */
    fun suche(text: String): List<Datenbank.Treffer> {
        val woerter = text.lowercase(Locale.GERMAN).split(' ').filter { it.isNotBlank() }
        if (woerter.isEmpty()) return emptyList()
        return liste.filter { t ->
            val heu = "${t.name} ${t.marke}".lowercase(Locale.GERMAN)
            woerter.all { it in heu }
        }
    }

    fun zuBarcode(code: String): Datenbank.Treffer? =
        liste.firstOrNull { it.barcode.isNotEmpty() && it.barcode == code }

    companion object {
        /** Liest ein Lebensmittel so, wie der Rechner es schickt. */
        fun ausJson(o: JSONObject): Datenbank.Treffer? {
            val name = o.optString("name").trim()
            val je100 = Naehrwerte.ausJson(o.optJSONObject("je100") ?: JSONObject())
            if (name.isEmpty() || je100.kcal <= 0) return null
            return Datenbank.Treffer(
                barcode = o.optString("barcode"),
                name = name,
                marke = o.optString("marke"),
                menge = "",
                je100 = je100,
                einheit = o.optString("einheit", "g").ifEmpty { "g" },
                quelle = "eigen",
            )
        }
    }
}
