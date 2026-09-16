package de.esstracker.android

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.content.ContextCompat

/**
 * Die Erlaubnis, ins Heimnetz zu kommen.
 *
 * Ab Android 16 liegen Adressen aus privaten Netzen hinter einer eigenen,
 * gefaehrlich eingestuften Erlaubnis. `INTERNET` allein genuegt nicht mehr.
 * Fehlt sie, laeuft jede Verbindung zum Rechner in einen Zeitablauf — und
 * zwar ohne dass irgendwo steht, warum. Genau deshalb steht das hier in einer
 * eigenen Datei und nicht beilaeufig in einem Bildschirm: es ist der
 * haeufigste Grund, warum diese App nichts uebertraegt.
 *
 * Auf aelteren Fassungen gibt es die Erlaubnis nicht. Dort ist alles schon
 * erlaubt, und es darf nicht danach gefragt werden — eine Abfrage nach einer
 * unbekannten Erlaubnis kommt sofort als „abgelehnt" zurueck und wuerde die
 * App auf jedem aelteren Geraet blockieren.
 */
object Heimnetz {

    const val ERLAUBNIS = "android.permission.ACCESS_LOCAL_NETWORK"

    /** Ob dieses Android die Erlaubnis ueberhaupt kennt. */
    val wirdVerlangt: Boolean
        get() = Build.VERSION.SDK_INT >= 36

    /** Ob die App gerade ins Heimnetz darf. */
    fun erlaubt(zusammenhang: Context): Boolean {
        if (!wirdVerlangt) return true
        return ContextCompat.checkSelfPermission(zusammenhang, ERLAUBNIS) ==
            PackageManager.PERMISSION_GRANTED
    }
}
