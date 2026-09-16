package de.esstracker.android

import android.content.Context
import android.provider.Settings
import org.json.JSONArray
import org.json.JSONObject
import java.io.InterruptedIOException
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.HttpURLConnection
import java.net.Inet4Address
import java.net.InetAddress
import java.net.SocketTimeoutException
import java.net.URL

/**
 * Die Verbindung zum eigenen Rechner.
 *
 * Drei Dinge passieren hier: den Rechner finden, sich einmalig koppeln, und
 * danach die Warteschlange leeren.
 *
 * Die Suche laeuft ueber einen UDP-Rundruf ins eigene Netz. Der Rechner
 * antwortet mit seiner Adresse — deshalb muss hier nie eine IP eingetippt
 * werden, und deshalb funktioniert die App auch noch, wenn der Router dem
 * Rechner morgen eine andere Adresse gibt.
 *
 * Ein Token bekommt das Handy nur, solange am Rechner ein Kopplungsfenster
 * offen steht. Danach liegt es in den Einstellungen dieser App und geht bei
 * jeder Uebertragung mit.
 */
class Rechner(private val zusammenhang: Context) {

    private val einstellungen =
        zusammenhang.getSharedPreferences("rechner", Context.MODE_PRIVATE)

    var adresse: String?
        get() = einstellungen.getString("adresse", null)
        private set(wert) = einstellungen.edit().putString("adresse", wert).apply()

    var token: String?
        get() = einstellungen.getString("token", null)
        private set(wert) = einstellungen.edit().putString("token", wert).apply()

    var name: String?
        get() = einstellungen.getString("name", null)
        private set(wert) = einstellungen.edit().putString("name", wert).apply()

    val gekoppelt: Boolean get() = !token.isNullOrEmpty() && !adresse.isNullOrEmpty()

    /**
     * Die Kennung dieses Handys. Sie bleibt ueber Neustarts gleich, damit der
     * Rechner beim zweiten Koppeln dasselbe Geraet wiedererkennt und nicht
     * eine zweite Zeile anlegt.
     */
    @Suppress("HardwareIds")
    val geraeteId: String by lazy {
        Settings.Secure.getString(zusammenhang.contentResolver, Settings.Secure.ANDROID_ID)
            ?: "unbekannt"
    }

    val geraeteName: String get() = "${android.os.Build.MANUFACTURER} ${android.os.Build.MODEL}".trim()

    fun vergiss() {
        einstellungen.edit().clear().apply()
    }

    // ---------- Suchen ----------

    /** Was die Suche gefunden hat. */
    data class Fund(
        val adresse: String,
        val rechner: String,
        val kopplungOffen: Boolean,
        val wort: String,
    )

    /**
     * Ruft ins Heimnetz und sammelt, was antwortet.
     *
     * Gerufen wird an die Rundrufadresse. Zwei Sekunden warten reicht in
     * jedem Heimnetz; laenger zu warten macht die Suche nur zaeh, ohne mehr
     * zu finden.
     */
    fun suche(dauerMs: Int = 2000): List<Fund> {
        val funde = mutableMapOf<String, Fund>()
        var buchse: DatagramSocket? = null
        try {
            buchse = DatagramSocket().apply {
                broadcast = true
                soTimeout = 300
            }
            val ruf = RUF.toByteArray()
            for (ziel in rundrufziele()) {
                try {
                    buchse.send(DatagramPacket(ruf, ruf.size, ziel, PORT))
                } catch (fehler: Exception) {
                    // Eine Schnittstelle, die keinen Rundruf annimmt, ist kein
                    // Grund aufzuhoeren — die naechste tut es vielleicht.
                }
            }

            val bis = System.currentTimeMillis() + dauerMs
            val puffer = ByteArray(1024)
            while (System.currentTimeMillis() < bis) {
                val paket = DatagramPacket(puffer, puffer.size)
                try {
                    buchse.receive(paket)
                } catch (fehler: SocketTimeoutException) {
                    continue
                } catch (fehler: InterruptedIOException) {
                    continue
                }
                val antwort = try {
                    JSONObject(String(paket.data, 0, paket.length))
                } catch (fehler: Exception) {
                    continue
                }
                if (antwort.optString("dienst") != "esstracker") continue

                val ip = paket.address.hostAddress ?: continue
                val port = antwort.optInt("port", PORT)
                funde[ip] = Fund(
                    adresse = "$ip:$port",
                    rechner = antwort.optString("rechner", ip),
                    kopplungOffen = antwort.optBoolean("kopplung", false),
                    wort = antwort.optString("wort", ""),
                )
            }
        } catch (fehler: Exception) {
            // Kein Netz, kein Rundruf: die Liste bleibt leer, und der
            // Bildschirm bietet die Adresse von Hand an.
        } finally {
            buchse?.close()
        }
        return funde.values.toList()
    }

    /**
     * Die Adressen, an die gerufen wird.
     *
     * 255.255.255.255 allein genuegt nicht: manche Geraete und manche Router
     * lassen den allgemeinen Rundruf fallen, den gerichteten aber durch.
     * Deshalb kommt zu jeder eigenen IPv4-Adresse auch die Rundrufadresse
     * ihres Netzes dazu.
     */
    private fun rundrufziele(): List<InetAddress> {
        val ziele = mutableListOf<InetAddress>()
        try {
            ziele.add(InetAddress.getByName("255.255.255.255"))
        } catch (fehler: Exception) {
            // dann eben nur die gerichteten
        }
        try {
            for (schnitt in java.net.NetworkInterface.getNetworkInterfaces()) {
                if (!schnitt.isUp || schnitt.isLoopback) continue
                for (adresse in schnitt.interfaceAddresses) {
                    val rundruf = adresse.broadcast ?: continue
                    ziele.add(rundruf)
                }
            }
        } catch (fehler: Exception) {
            // dann eben nur der allgemeine
        }
        return ziele.distinct()
    }

    /** Fragt eine Adresse direkt — der Weg, wenn der Rundruf nicht durchkommt. */
    fun frage(adresseMitPort: String): Fund? {
        val adresse = mitPort(adresseMitPort)
        val roh = hole("http://$adresse/hallo", "GET", null, null) ?: return null
        return try {
            val o = JSONObject(roh)
            if (o.optString("dienst") != "esstracker") return null
            Fund(
                adresse = adresse,
                rechner = o.optString("rechner", adresse),
                kopplungOffen = o.optBoolean("kopplung", false),
                wort = o.optString("wort", ""),
            )
        } catch (fehler: Exception) {
            null
        }
    }

    // ---------- Koppeln ----------

    sealed class Ergebnis {
        data class Fertig(val rechner: String, val ziel: Int) : Ergebnis()
        data class Schiefgelaufen(val grund: String) : Ergebnis()
    }

    /** Holt ein Token. Geht nur, solange am Rechner das Fenster offen steht. */
    fun kopple(adresseMitPort: String): Ergebnis {
        val adresse = mitPort(adresseMitPort)
        if (!istPrivat(adresse)) {
            return Ergebnis.Schiefgelaufen(
                "$adresse liegt nicht im Heimnetz. Diese App überträgt nur an Adressen im eigenen Netz."
            )
        }
        val rumpf = JSONObject()
            .put("geraet", geraeteId)
            .put("name", geraeteName)
            .toString()

        val antwort = hole("http://$adresse/koppeln", "POST", rumpf, null)
            ?: return Ergebnis.Schiefgelaufen(
                "Der Rechner antwortet nicht. Steht das Kopplungsfenster dort offen?"
            )
        return try {
            val o = JSONObject(antwort)
            o.optString("fehler").takeIf { it.isNotEmpty() }?.let {
                return Ergebnis.Schiefgelaufen(it)
            }
            val neuesToken = o.optString("token")
            if (neuesToken.isEmpty()) {
                return Ergebnis.Schiefgelaufen("Der Rechner hat kein Token geschickt.")
            }
            this.adresse = adresse
            this.token = neuesToken
            this.name = o.optString("rechner", adresse)
            Ergebnis.Fertig(this.name ?: adresse, o.optInt("ziel", 2000))
        } catch (fehler: Exception) {
            Ergebnis.Schiefgelaufen("Die Antwort des Rechners war unverständlich.")
        }
    }

    // ---------- Senden ----------

    /** Was beim Senden herauskam. */
    data class Abrechnung(
        val angenommen: List<String>,
        val neu: Int,
        val ziel: Int?,
        val grund: String?,
    ) {
        val geklappt: Boolean get() = grund == null
    }

    /**
     * Schickt die Warteschlange.
     *
     * Alles in einem Paket: beim Aufholen nach einem ausgeschalteten Rechner
     * sind das schnell zwanzig Eintraege, und zwanzig einzelne Anfragen ueber
     * WLAN dauern spuerbar laenger als eine.
     *
     * Der Rechner meldet zurueck, welche IDs bei ihm liegen — auch die, die er
     * schon kannte. Genau das macht das Nachschicken gefahrlos.
     */
    fun sende(stapel: List<Eintrag>): Abrechnung {
        if (stapel.isEmpty()) return Abrechnung(emptyList(), 0, null, null)
        val adresse = adresse ?: return Abrechnung(emptyList(), 0, null, "Kein Rechner gekoppelt.")
        val token = token ?: return Abrechnung(emptyList(), 0, null, "Kein Token — bitte neu koppeln.")

        val liste = JSONArray()
        // In Haeppchen: ein Paket ist beim Rechner auf 500 Eintraege und ein
        // Megabyte gedeckelt, und ein Handy, das drei Wochen offline war, kann
        // darueber kommen.
        val teil = stapel.take(200)
        teil.forEach { liste.put(it.alsJson()) }

        val antwort = hole("http://$adresse/aufnahme", "POST", liste.toString(), token)
            ?: return Abrechnung(emptyList(), 0, null, "Der Rechner ist nicht erreichbar.")

        return try {
            val o = JSONObject(antwort)
            o.optString("fehler").takeIf { it.isNotEmpty() }?.let {
                return Abrechnung(emptyList(), 0, null, it)
            }
            val ids = o.optJSONArray("angenommen") ?: JSONArray()
            Abrechnung(
                angenommen = (0 until ids.length()).map { ids.getString(it) },
                neu = o.optInt("neu", 0),
                ziel = o.optInt("ziel").takeIf { it > 0 },
                grund = null,
            )
        } catch (fehler: Exception) {
            Abrechnung(emptyList(), 0, null, "Die Antwort des Rechners war unverständlich.")
        }
    }

    // ---------- Werkzeug ----------

    private fun mitPort(eingabe: String): String {
        val rein = eingabe.trim().removePrefix("http://").removePrefix("https://").trimEnd('/')
        return if (rein.contains(':')) rein else "$rein:$PORT"
    }

    /**
     * Prueft, ob eine Adresse im privaten Netz liegt.
     *
     * Das ist der eigentliche Riegel gegen ungewolltes Versenden: die
     * Netzfreigabe der App erlaubt unverschluesseltes HTTP, weil die Adresse
     * des Rechners erst zur Laufzeit feststeht. Damit daraus kein Weg wird,
     * Essenstagebuecher im Klartext an einen fremden Server zu schicken, kommt
     * hier nur durch, was nach RFC 1918 privat, Link-Local oder Loopback ist —
     * und der Bereich, den Tailscale benutzt.
     */
    private fun istPrivat(adresseMitPort: String): Boolean {
        val wirt = adresseMitPort.substringBeforeLast(':')
        return try {
            val ip = InetAddress.getByName(wirt)
            if (ip !is Inet4Address) return ip.isSiteLocalAddress || ip.isLoopbackAddress || ip.isLinkLocalAddress
            val b = ip.address.map { it.toInt() and 0xFF }
            when {
                b[0] == 10 -> true                                  // 10.0.0.0/8
                b[0] == 172 && b[1] in 16..31 -> true               // 172.16.0.0/12
                b[0] == 192 && b[1] == 168 -> true                  // 192.168.0.0/16
                b[0] == 127 -> true                                 // Loopback
                b[0] == 169 && b[1] == 254 -> true                  // Link-Local
                b[0] == 100 && b[1] in 64..127 -> true              // 100.64.0.0/10, u.a. Tailscale
                else -> false
            }
        } catch (fehler: Exception) {
            false
        }
    }

    private fun hole(adresse: String, art: String, rumpf: String?, token: String?): String? {
        var verbindung: HttpURLConnection? = null
        return try {
            verbindung = (URL(adresse).openConnection() as HttpURLConnection).apply {
                requestMethod = art
                connectTimeout = 4000
                readTimeout = 10000
                setRequestProperty("Accept", "application/json")
                token?.let { setRequestProperty("Authorization", "Bearer $it") }
                if (rumpf != null) {
                    doOutput = true
                    setRequestProperty("Content-Type", "application/json")
                    outputStream.use { it.write(rumpf.toByteArray()) }
                }
            }
            val strom = if (verbindung.responseCode in 200..299) {
                verbindung.inputStream
            } else {
                // Der Fehlertext des Rechners ist brauchbar („am Rechner ist
                // gerade kein Kopplungsfenster offen") und soll nicht durch ein
                // allgemeines „ging nicht“ ersetzt werden.
                verbindung.errorStream ?: return null
            }
            strom.bufferedReader().use { it.readText() }
        } catch (fehler: Exception) {
            android.util.Log.w("Esstracker", "hole($adresse) scheiterte", fehler)
            null
        } finally {
            verbindung?.disconnect()
        }
    }

    companion object {
        const val PORT = 8790
        const val RUF = "esstracker-wo-bist-du"
    }
}
