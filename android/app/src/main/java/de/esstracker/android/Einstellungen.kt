package de.esstracker.android

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Computer
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Der Rechner-Bildschirm.
 *
 * Hier passiert genau zweierlei: koppeln und nachsehen, ob alles drüben ist.
 * Das Tagesziel steht auch hier, aber nur zur Ansicht — gesetzt wird es auf
 * dem Rechner, damit es nicht zwei Wahrheiten gibt, die sich gegenseitig
 * ueberschreiben.
 */
@Composable
fun EinstellungenBildschirm(zustand: Zustand) {
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item { if (zustand.gekoppelt) Gekoppelt(zustand) else Koppeln(zustand) }
        item { ZielKarte(zustand) }
        item { UeberKarte() }
    }
}

@Composable
private fun Gekoppelt(zustand: Zustand) {
    Karte {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                Icons.Outlined.Computer, contentDescription = null,
                tint = Akzent, modifier = Modifier.size(20.dp),
            )
            Breite(10)
            Column(Modifier.weight(1f)) {
                Text(
                    zustand.rechnerName ?: "Rechner",
                    fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                )
                Text(
                    zustand.rechner.adresse ?: "",
                    fontSize = 11.5.sp, color = Leise, fontFamily = Mono,
                )
            }
        }
        Luft(14)
        Trennlinie()
        Luft(14)

        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(
                    when (zustand.offen) {
                        0 -> "Alles übertragen"
                        1 -> "1 Eintrag wartet"
                        else -> "${zustand.offen} Einträge warten"
                    },
                    fontSize = 13.5.sp, fontWeight = FontWeight.Medium, color = Tinte,
                )
                Text(
                    "Jeder Eintrag geht mit allen Nährwerten rüber — Produkt, Marke, " +
                        "Strichcode, Menge, Uhrzeit.",
                    fontSize = 11.5.sp, color = Leise,
                )
            }
            Breite(10)
            Knopf(
                if (zustand.sendetGerade) "…" else "Senden",
                haupt = zustand.offen > 0,
                an = !zustand.sendetGerade,
            ) { zustand.uebertrage() }
        }
        Luft(14)
        Knopf("Rechner trennen", modifier = Modifier.fillMaxWidth()) { zustand.trenne() }
    }
}

@Composable
private fun Koppeln(zustand: Zustand) {
    var vonHand by remember { mutableStateOf("") }
    val zusammenhang = LocalContext.current
    var darfHeimnetz by remember { mutableStateOf(Heimnetz.erlaubt(zusammenhang)) }

    // Die Erlaubnis wird genau dann erfragt, wenn sie gebraucht wird — beim
    // ersten Griff nach dem Rechner. Vorher waere sie eine Frage ohne
    // sichtbaren Anlass, und solche Fragen werden weggetippt.
    val frage = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { erteilt ->
        darfHeimnetz = erteilt
        if (erteilt) zustand.sucheRechner()
        else zustand.sage("Ohne Zugriff aufs Heimnetz kommt die App nicht an den Rechner.")
    }

    fun mitErlaubnis(tat: () -> Unit) {
        if (darfHeimnetz) tat() else frage.launch(Heimnetz.ERLAUBNIS)
    }

    Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
    if (!darfHeimnetz) {
        Karte {
            KartenKopf("Zugriff aufs Heimnetz fehlt")
            Text(
                "Ab Android 16 braucht jede App eine eigene Erlaubnis, um Geräte im " +
                    "eigenen Netz zu erreichen. Ohne sie läuft jeder Versuch, den Rechner " +
                    "zu finden, stumm in einen Zeitablauf.",
                fontSize = 12.5.sp, color = Gedaempft,
            )
            Luft(12)
            Knopf("Zugriff erlauben", haupt = true, modifier = Modifier.fillMaxWidth()) {
                frage.launch(Heimnetz.ERLAUBNIS)
            }
        }
    }

    Karte {
        KartenKopf("Rechner koppeln")
        Text(
            "Am Rechner im Esstracker auf „Handy“ → „Kopplung öffnen“. Dann hier suchen. " +
                "Das Erkennungswort muss auf beiden Seiten gleich sein.",
            fontSize = 12.5.sp, color = Gedaempft,
        )
        Luft(14)

        if (zustand.suchtRechner) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                CircularProgressIndicator(
                    color = Akzent, strokeWidth = 2.dp, modifier = Modifier.size(18.dp)
                )
                Breite(10)
                Text("sucht im WLAN …", fontSize = 13.sp, color = Gedaempft)
            }
        } else {
            Knopf("Rechner suchen", haupt = true, modifier = Modifier.fillMaxWidth()) {
                mitErlaubnis { zustand.sucheRechner() }
            }
        }

        if (zustand.funde.isNotEmpty()) {
            Luft(14)
            for (fund in zustand.funde) {
                Fundzeile(fund) { zustand.kopple(fund.adresse) }
                Luft(8)
            }
        }

        Luft(14)
        Trennlinie()
        Luft(14)
        Text(
            "Kommt die Suche nicht durch — manche Router sperren den Rundruf —, " +
                "steht die Adresse am Rechner unter „Handy“:",
            fontSize = 12.sp, color = Leise,
        )
        Luft(8)
        Row(verticalAlignment = Alignment.CenterVertically) {
            Feld(
                wert = vonHand,
                aufWert = { vonHand = it },
                platzhalter = "192.168.1.20:8790",
                modifier = Modifier.weight(1f),
                tastatur = KeyboardType.Uri,
                aufFertig = { if (vonHand.isNotBlank()) mitErlaubnis { zustand.frageAdresse(vonHand) } },
            )
            Breite(8)
            Knopf("Fragen", an = vonHand.isNotBlank()) { mitErlaubnis { zustand.frageAdresse(vonHand) } }
        }
    }
    }
}

@Composable
private fun Fundzeile(fund: Rechner.Fund, tat: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(FlaecheHoch, RoundedCornerShape(10.dp))
            .border(if (fund.kopplungOffen) Akzent else Rahmen, 10.dp)
            .clickable(enabled = fund.kopplungOffen) { tat() }
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(fund.rechner, fontSize = 14.sp, fontWeight = FontWeight.Medium, color = Tinte)
            Text(fund.adresse, fontSize = 11.5.sp, color = Leise, fontFamily = Mono)
            if (fund.kopplungOffen && fund.wort.isNotEmpty()) {
                Luft(6)
                // Das Wort ist der ganze Abgleich: steht am Rechner dasselbe,
                // ist es der richtige. Es schuetzt nichts — es unterscheidet.
                Text(
                    fund.wort,
                    fontSize = 17.sp, fontWeight = FontWeight.SemiBold,
                    color = Akzent, fontFamily = Mono,
                )
            }
        }
        Breite(10)
        if (fund.kopplungOffen) {
            Knopf("Koppeln", haupt = true) { tat() }
        } else {
            Text(
                "Kopplung\nnicht offen",
                fontSize = 11.sp, color = Leise,
                textAlign = androidx.compose.ui.text.style.TextAlign.End,
            )
        }
    }
}

@Composable
private fun ZielKarte(zustand: Zustand) {
    Karte {
        KartenKopf("Tagesziel", "wird am Rechner gesetzt")
        Row(verticalAlignment = Alignment.Bottom) {
            Text(
                zustand.ziel.toString(),
                fontSize = 30.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                fontFamily = Mono,
            )
            Breite(6)
            Text(
                "kcal pro Tag", fontSize = 13.sp, color = Leise,
                modifier = Modifier.padding(bottom = 5.dp),
            )
        }
        Luft(8)
        Text(
            "Es kommt bei jeder Übertragung vom Rechner mit. Zwei Stellen, an denen " +
                "es sich ändern lässt, wären zwei Stellen, die sich widersprechen können.",
            fontSize = 11.5.sp, color = Leise,
        )
    }
}

@Composable
private fun UeberKarte() {
    Karte {
        KartenKopf("Woher die Daten kommen")
        Text(
            "Die Produkte stammen aus Open Food Facts — einer offenen Sammlung, an der " +
                "jeder mitschreibt. Nährwerte können deshalb fehlen oder falsch sein; " +
                "Produkte ohne Energieangabe blendet die Suche aus.",
            fontSize = 12.sp, color = Gedaempft,
        )
        Luft(10)
        Text(
            "Zum Rechner geht nichts über das Internet: die Übertragung läuft direkt " +
                "im Heimnetz, und die App überträgt ausschließlich an Adressen aus " +
                "privaten Netzbereichen.",
            fontSize = 12.sp, color = Gedaempft,
        )
    }
}
