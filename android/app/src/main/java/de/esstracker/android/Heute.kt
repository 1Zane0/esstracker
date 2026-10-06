package de.esstracker.android

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CloudDone
import androidx.compose.material.icons.outlined.CloudUpload
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.util.Locale

/**
 * Der Heute-Bildschirm — der Tracker.
 *
 * Oben die eine Zahl, darunter was sie ergeben hat. Mehr steht hier nicht:
 * keine Naehrwerte, keine Wochenkurve, kein Raster. Das alles kommt beim
 * Rechner an und wird dort gezeigt; hier wuerde es den Blick von der einen
 * Zahl wegziehen, wegen der man die App aufmacht.
 */
@Composable
fun HeuteBildschirm(zustand: Zustand) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item { Zaehler(zustand) }
        item { Uebertragung(zustand) }

        if (zustand.heute.isEmpty()) {
            item {
                Karte {
                    Leerzeile("Heute noch nichts eingetragen. Der Knopf „Eintragen“ unten führt zum Scanner.")
                }
            }
        } else {
            // Nach Mahlzeit gebuendelt, in fester Reihenfolge — beim Blick auf
            // den Tag sucht man „was war zum Mittag“, nicht „was war um 12:40“.
            for ((art, name) in MahlzeitNamen) {
                val zeilen = zustand.heute.filter { it.mahlzeit == art }
                if (zeilen.isEmpty()) continue
                item(key = "kopf-$art") {
                    MahlzeitKopf(name, zeilen.sumOf { it.kcal() }.toInt())
                }
                items(zeilen, key = { it.id }) { eintrag ->
                    Eintragszeile(eintrag, zustand)
                }
            }
        }
    }
}

/**
 * Die eine Zahl.
 *
 * Aufgebaut wie der Puls in der Seitenleiste auf dem Rechner: grosse Zahl,
 * daneben klein das Ziel, darunter der Balken und was noch uebrig ist.
 */
@Composable
private fun Zaehler(zustand: Zustand) {
    val kcal = zustand.kcalHeute
    val ziel = zustand.ziel
    val rest = ziel - kcal

    Karte {
        Text(
            LocalDate.now().format(DateTimeFormatter.ofPattern("EEEE, d. MMMM", Locale.GERMAN)),
            fontSize = 12.sp, color = Leise,
        )
        Luft(10)
        Row(verticalAlignment = Alignment.Bottom) {
            Text(
                kcal.toString(),
                fontSize = 52.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                letterSpacing = (-2).sp,
            )
            Breite(8)
            Text(
                "von $ziel kcal",
                fontSize = 14.sp, color = Leise,
                modifier = Modifier.padding(bottom = 9.dp),
            )
        }
        Luft(12)
        Balken(if (ziel > 0) kcal / ziel.toFloat() else 0f, hoehe = 5)
        Luft(10)
        Text(
            when {
                zustand.heute.isEmpty() -> "noch nichts heute"
                rest >= 0 -> "$rest kcal übrig"
                else -> "${-rest} kcal darüber"
            } + when (val offen = zustand.offenHeute) {
                // Die Zahl oben ist dann zu niedrig — das soll dastehen, bevor
                // jemand ihr glaubt.
                0 -> ""
                1 -> " · 1 Eintrag ohne Nährwerte"
                else -> " · $offen ohne Nährwerte"
            },
            fontSize = 12.5.sp,
            color = if (zustand.heute.isEmpty()) Leise else Gedaempft,
        )
    }
}

/**
 * Der Stand der Uebertragung.
 *
 * Er steht direkt unter der Zahl und nicht in den Einstellungen: dass ein
 * Eintrag noch nicht beim Rechner liegt, ist kein Fehler, aber es ist etwas,
 * das man sehen will, ohne danach zu suchen.
 */
@Composable
private fun Uebertragung(zustand: Zustand) {
    val offen = zustand.offen
    val gekoppelt = zustand.gekoppelt

    Karte {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                if (offen == 0 && gekoppelt) Icons.Outlined.CloudDone else Icons.Outlined.CloudUpload,
                contentDescription = null,
                tint = if (offen == 0 && gekoppelt) Gedaempft else Tinte,
                modifier = Modifier.size(18.dp),
            )
            Breite(10)
            Column(Modifier.weight(1f)) {
                Text(
                    when {
                        !gekoppelt -> "Kein Rechner gekoppelt"
                        offen == 0 -> "Alles übertragen"
                        offen == 1 -> "1 Eintrag wartet"
                        else -> "$offen Einträge warten"
                    },
                    fontSize = 13.5.sp, fontWeight = FontWeight.Medium, color = Tinte,
                )
                Text(
                    when {
                        !gekoppelt -> "Unter „Rechner“ koppeln"
                        offen == 0 -> zustand.rechnerName ?: "—"
                        else -> "gehen raus, sobald der Rechner erreichbar ist"
                    },
                    fontSize = 11.5.sp, color = Leise,
                )
            }
            if (gekoppelt && offen > 0) {
                Knopf(
                    if (zustand.sendetGerade) "…" else "Jetzt",
                    haupt = true,
                    an = !zustand.sendetGerade,
                ) { zustand.uebertrage() }
            }
        }
    }
}

@Composable
private fun MahlzeitKopf(name: String, kcal: Int) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(start = 4.dp, end = 4.dp, top = 6.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(
            name.uppercase(Locale.GERMAN),
            fontSize = 11.sp, fontWeight = FontWeight.SemiBold,
            color = Leise, letterSpacing = 0.8.sp,
        )
        Text("$kcal kcal", fontSize = 11.5.sp, color = Leise, fontFamily = Mono)
    }
}

@Composable
private fun Eintragszeile(eintrag: Eintrag, zustand: Zustand) {
    var offen by remember { mutableStateOf(false) }

    Column(
        Modifier
            .fillMaxWidth()
            .background(Flaeche, RoundedCornerShape(10.dp))
            .border(Rahmen, 10.dp)
            .clickable { offen = !offen }
            .padding(horizontal = 14.dp, vertical = 11.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(eintrag.uhr(), fontSize = 11.sp, color = Leise, fontFamily = Mono)
            Breite(12)
            Column(Modifier.weight(1f)) {
                Text(
                    eintrag.name,
                    fontSize = 14.sp, fontWeight = FontWeight.Medium, color = Tinte,
                    maxLines = 1,
                    overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                )
                Text(
                    buildString {
                        if (eintrag.marke.isNotEmpty()) append(eintrag.marke).append(" · ")
                        append("${eintrag.menge.toInt()} ${eintrag.einheit}")
                        if (eintrag.offen) append(" · Werte folgen")
                    },
                    fontSize = 11.5.sp, color = Leise,
                    maxLines = 1,
                    overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                )
            }
            Breite(10)
            Text(
                if (eintrag.offen) "?" else "${eintrag.kcal().toInt()}",
                fontSize = 15.sp, fontWeight = FontWeight.SemiBold,
                color = if (eintrag.offen) Leise else Tinte,
                fontFamily = Mono,
            )
            Text(" kcal", fontSize = 11.sp, color = Leise)
        }

        // Aufgeklappt steht da, woraus die Zahl entstanden ist, und ob der
        // Eintrag schon beim Rechner liegt. Zugeklappt waere beides Ballast.
        if (offen) {
            Luft(10)
            Trennlinie()
            Luft(10)
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(
                        when {
                            eintrag.offen -> "Nährwerte schätzt Claude am Rechner"
                            eintrag.geschaetzt -> "${eintrag.je100.kcal.toInt()} kcal je 100 ${eintrag.einheit} · geschätzt"
                            else -> "${eintrag.je100.kcal.toInt()} kcal je 100 ${eintrag.einheit}"
                        } + if (eintrag.barcode.isNotEmpty()) " · ${eintrag.barcode}" else "",
                        fontSize = 11.5.sp, color = Gedaempft, fontFamily = Mono,
                    )
                    Text(
                        if (eintrag.gesendet) "auf dem Rechner" else "wartet auf Übertragung",
                        fontSize = 11.5.sp, color = Leise,
                    )
                }
                IconButton(onClick = { zustand.loesche(eintrag) }) {
                    Icon(
                        Icons.Outlined.Delete, contentDescription = "Löschen",
                        tint = Gedaempft, modifier = Modifier.size(18.dp),
                    )
                }
            }
        }
    }
}

/**
 * Ein Knopf in den Kanten der Tasklist: heller Kasten mit schwarzer Schrift
 * fuer den Hauptweg, sonst Rahmen auf dunkler Flaeche.
 */
@Composable
fun Knopf(
    beschriftung: String,
    haupt: Boolean = false,
    an: Boolean = true,
    modifier: Modifier = Modifier,
    tat: () -> Unit,
) {
    val grund = if (haupt) Akzent else Flaeche
    val schrift = if (haupt) Grund else Tinte
    Box(
        modifier
            .background(if (an) grund else FlaecheHoch, RoundedCornerShape(8.dp))
            .then(if (haupt) Modifier else Modifier.border(Rahmen, 8.dp))
            .clickable(enabled = an) { tat() }
            .padding(horizontal = 14.dp, vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            beschriftung,
            fontSize = 13.sp,
            fontWeight = if (haupt) FontWeight.SemiBold else FontWeight.Medium,
            color = if (an) schrift else Leise,
        )
    }
}
