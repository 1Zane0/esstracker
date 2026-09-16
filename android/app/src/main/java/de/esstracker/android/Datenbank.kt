package de.esstracker.android

import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

/**
 * Open Food Facts — die Produktdatenbank.
 *
 * Eine offene Sammlung von Freiwilligen, rund vier Millionen Produkte, viele
 * davon mit deutschem Namen. Genutzt werden zwei Wege:
 *
 *  - **Strichcode**: `api/v2/product/<code>.json`. Der genaue Treffer.
 *  - **Text**: `search.openfoodfacts.org/search`. Das ist der Nachfolger des
 *    alten `cgi/search.pl`, das fuer Abfragen wie diese abgeschaltet wurde und
 *    nur noch eine Wartungsseite zurueckgibt.
 *
 * Aufgerufen wird ueber HttpURLConnection statt ueber eine Bibliothek: es sind
 * zwei Anfragen, beide GET, beide JSON. Eine Abhaengigkeit mehr waere hier
 * teurer als die dreissig Zeilen.
 *
 * Wichtig fuer die Datenbank: Open Food Facts bittet darum, dass sich Programme
 * im User-Agent zu erkennen geben. Das ist keine Formalie, sondern der Weg,
 * wie dort ein Programm gedrosselt wird, statt alle zu sperren.
 */
object Datenbank {

    private const val KENNUNG = "Esstracker/1.0 (Android; privates Ernaehrungstagebuch)"
    private const val PRODUKT = "https://world.openfoodfacts.org/api/v2/product/"
    private const val SUCHE = "https://search.openfoodfacts.org/search"

    /** Die Felder, die geholt werden — alles andere waere Ballast auf dem Mobilfunk. */
    private const val FELDER =
        "code,product_name,product_name_de,generic_name,brands,quantity,serving_size,nutriments"

    /**
     * Ein Treffer, noch ohne Menge.
     *
     * `quelle` sagt, woher er stammt: „off" aus der Datenbank, „hand" vom
     * Menschen selbst eingetippt. Der Rechner bekommt das mit und zeigt es in
     * der CSV — sonst liesse sich spaeter nicht mehr unterscheiden, welche
     * Naehrwerte nachgeschlagen und welche geschaetzt sind.
     */
    data class Treffer(
        val barcode: String,
        val name: String,
        val marke: String,
        val menge: String,
        val je100: Naehrwerte,
        val einheit: String,
        val quelle: String = "off",
    ) {
        /** Ob der Treffer brauchbar ist: ohne Energie ist er als Eintrag wertlos. */
        val hatEnergie: Boolean get() = je100.kcal > 0
    }

    /** Sucht ein Produkt ueber den Strichcode. */
    fun ueberBarcode(code: String): Treffer? {
        val roh = hole("$PRODUKT$code.json?fields=$FELDER") ?: return null
        val o = JSONObject(roh)
        // Die v2-Schnittstelle antwortet mit status 0 und leerem Produkt, wenn
        // der Code unbekannt ist — kein Fehler, nur kein Treffer.
        val produkt = o.optJSONObject("product") ?: return null
        return ausJson(produkt, code)
    }

    /** Sucht Produkte ueber den Namen. */
    fun ueberText(wort: String, anzahl: Int = 25): List<Treffer> {
        val wortRein = wort.trim()
        if (wortRein.length < 2) return emptyList()
        val adresse = "$SUCHE?q=${URLEncoder.encode(wortRein, "UTF-8")}" +
            "&page_size=$anzahl&fields=$FELDER"
        val roh = hole(adresse) ?: return emptyList()

        val hits: JSONArray = JSONObject(roh).optJSONArray("hits") ?: return emptyList()
        return (0 until hits.length())
            .mapNotNull { ausJson(hits.getJSONObject(it), null) }
            // Ohne Energie kann man nichts zaehlen. Solche Eintraege gibt es in
            // der Sammlung reichlich, und in einer Trefferliste sind sie nur
            // Zeilen, die beim Antippen nichts tun.
            .filter { it.hatEnergie }
    }

    // ---------- Auswerten ----------

    /**
     * Baut einen Treffer aus dem JSON.
     *
     * Zwei Eigenheiten der Sammlung, die hier abgefangen werden:
     *
     *  - `brands` ist bei der Produktabfrage eine Zeichenkette („Ritter Sport“),
     *    bei der Suche ein Feld (`["Ritter Sport"]`). Beides muss gehen.
     *  - Der Name kommt je nach Produkt unter `product_name_de`,
     *    `product_name` oder `generic_name`. Der deutsche zuerst.
     */
    private fun ausJson(o: JSONObject, barcodeFalls: String?): Treffer? {
        val name = ersterText(o, "product_name_de", "product_name", "generic_name")
            ?: return null

        val marke = when (val roh = o.opt("brands")) {
            is JSONArray -> (0 until roh.length()).joinToString(", ") { roh.getString(it) }
            is String -> roh
            else -> ""
        }.trim()

        val n = o.optJSONObject("nutriments") ?: JSONObject()
        val kcal = wert(n, "energy-kcal_100g", "energy-kcal")
        val kj = wert(n, "energy-kj_100g", "energy-kj", "energy_100g")

        val je100 = Naehrwerte(
            // Manche Produkte tragen nur Kilojoule. Der Umrechnungsfaktor ist
            // fest: 1 kcal = 4,184 kJ. Lieber gerechnet als eine Null zeigen.
            kcal = if (kcal > 0) kcal else kj / 4.184,
            kj = if (kj > 0) kj else kcal * 4.184,
            fett = wert(n, "fat_100g", "fat"),
            gesaettigt = wert(n, "saturated-fat_100g", "saturated-fat"),
            kohlenhydrate = wert(n, "carbohydrates_100g", "carbohydrates"),
            zucker = wert(n, "sugars_100g", "sugars"),
            ballaststoffe = wert(n, "fiber_100g", "fiber"),
            eiweiss = wert(n, "proteins_100g", "proteins"),
            salz = wert(n, "salt_100g", "salt"),
        )

        val menge = o.optString("quantity", "")
        return Treffer(
            barcode = o.optString("code", barcodeFalls ?: ""),
            name = name.trim(),
            marke = marke,
            menge = menge,
            je100 = je100,
            // Getraenke werden in Millilitern gemessen. Die Sammlung sagt das
            // nicht direkt — die Mengenangabe ist der beste Hinweis darauf.
            einheit = if (menge.contains("ml", true) || menge.contains(" l", true)) "ml" else "g",
        )
    }

    private fun ersterText(o: JSONObject, vararg schluessel: String): String? {
        for (s in schluessel) {
            val wert = o.optString(s, "").trim()
            if (wert.isNotEmpty()) return wert
        }
        return null
    }

    private fun wert(o: JSONObject, vararg schluessel: String): Double {
        for (s in schluessel) {
            if (o.has(s)) {
                val d = o.optDouble(s, Double.NaN)
                if (!d.isNaN() && d >= 0) return d
            }
        }
        return 0.0
    }

    // ---------- Holen ----------

    /**
     * Holt eine Adresse. Gibt null zurueck, wenn nichts Brauchbares kommt —
     * ein fehlender Treffer und ein fehlendes Netz sehen fuer den Aufrufer
     * gleich aus, und beides fuehrt zum selben Satz auf dem Bildschirm.
     */
    private fun hole(adresse: String): String? {
        // Das https steht hier fest und kommt nicht aus einer Einstellung:
        // die Netzfreigabe der App erlaubt Klartext, und ohne diese Stelle
        // liesse sich die Abfrage in ein unverschluesseltes http umbiegen.
        if (!adresse.startsWith("https://")) return null

        var verbindung: HttpURLConnection? = null
        return try {
            verbindung = (URL(adresse).openConnection() as HttpURLConnection).apply {
                requestMethod = "GET"
                setRequestProperty("User-Agent", KENNUNG)
                setRequestProperty("Accept", "application/json")
                connectTimeout = 8000
                readTimeout = 12000
            }
            if (verbindung.responseCode !in 200..299) return null
            verbindung.inputStream.bufferedReader().use { it.readText() }
        } catch (fehler: Exception) {
            null
        } finally {
            verbindung?.disconnect()
        }
    }
}
