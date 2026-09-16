package de.esstracker.android

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.material.icons.outlined.ArrowBack
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
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Der letzte Schritt: wie viel, und zu welcher Mahlzeit.
 *
 * Die Kalorien stehen dabei gross und rechnen bei jedem Tastendruck mit. Das
 * ist der Punkt, an dem die Zahl entsteht, und sie soll dort entstehen, wo man
 * sie beeinflusst — nicht erst auf dem Bildschirm danach.
 */
@Composable
fun MengeBildschirm(zustand: Zustand, treffer: Datenbank.Treffer) {
    // Die Vorgabe ist 100: das ist die Bezugsgroesse der Naehrwerte und damit
    // die Zahl, die am wenigsten ueberrascht. Eine geratene Portionsgroesse
    // waere bequemer und oefter falsch.
    var menge by remember(treffer) { mutableStateOf("100") }
    var mahlzeit by remember(treffer) { mutableStateOf(mahlzeitJetzt()) }

    val zahl = menge.replace(',', '.').toDoubleOrNull() ?: 0.0
    val kcal = (treffer.je100.kcal * zahl / 100.0).toInt()
    val gueltig = zahl > 0 && zahl <= 100000

    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item {
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = { zustand.gewaehlt = null }) {
                    Icon(
                        Icons.Outlined.ArrowBack, contentDescription = "Zurück",
                        tint = Gedaempft, modifier = Modifier.size(20.dp),
                    )
                }
                Text("Eintragen", fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = Tinte)
            }
        }

        item {
            Karte {
                Text(
                    treffer.name,
                    fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                )
                if (treffer.marke.isNotEmpty() || treffer.barcode.isNotEmpty()) {
                    Luft(3)
                    Text(
                        listOfNotNull(
                            treffer.marke.takeIf { it.isNotEmpty() },
                            treffer.menge.takeIf { it.isNotEmpty() },
                            treffer.barcode.takeIf { it.isNotEmpty() },
                        ).joinToString(" · "),
                        fontSize = 11.5.sp, color = Leise,
                    )
                }
                Luft(16)
                Row(verticalAlignment = Alignment.Bottom) {
                    Text(
                        kcal.toString(),
                        fontSize = 44.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                        letterSpacing = (-1.5).sp,
                    )
                    Breite(7)
                    Text(
                        "kcal", fontSize = 14.sp, color = Leise,
                        modifier = Modifier.padding(bottom = 8.dp),
                    )
                }
                Luft(4)
                Text(
                    "${treffer.je100.kcal.toInt()} kcal je 100 ${treffer.einheit}",
                    fontSize = 11.5.sp, color = Leise, fontFamily = Mono,
                )
            }
        }

        item {
            Karte {
                KartenKopf("Menge", "in ${treffer.einheit}")
                Feld(
                    wert = menge,
                    aufWert = { neu -> menge = neu.filter { it.isDigit() || it == ',' || it == '.' } },
                    platzhalter = "100",
                    modifier = Modifier.fillMaxWidth(),
                    tastatur = KeyboardType.Number,
                )
                Luft(10)
                // Die vier Griffe decken ab, was man beim Essen wirklich
                // abschaetzt. Wer genauer sein will, tippt — aber meistens
                // will man das nicht.
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (vorgabe in listOf(30, 50, 100, 250)) {
                        Knopf("$vorgabe", modifier = Modifier.weight(1f)) { menge = vorgabe.toString() }
                    }
                }
            }
        }

        item {
            Karte {
                KartenKopf("Mahlzeit", "aus der Uhrzeit geraten")
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for ((art, name) in MahlzeitKurz) {
                        Wahl(
                            name,
                            gewaehlt = mahlzeit == art,
                            modifier = Modifier.weight(1f),
                        ) { mahlzeit = art }
                    }
                }
            }
        }

        item {
            Knopf(
                if (gueltig) "Eintragen · $kcal kcal" else "Menge fehlt",
                haupt = true,
                an = gueltig,
                modifier = Modifier.fillMaxWidth(),
            ) {
                zustand.trageEin(
                    Eintrag(
                        name = treffer.name,
                        marke = treffer.marke,
                        barcode = treffer.barcode,
                        menge = zahl,
                        einheit = treffer.einheit,
                        je100 = treffer.je100,
                        mahlzeit = mahlzeit,
                        quelle = treffer.quelle,
                    )
                )
                zustand.gewaehlt = null
                zustand.treffer.clear()
                zustand.suchtext = ""
                zustand.seite = Seite.Heute
            }
        }
    }
}

/** Ein Knopf, der an bleibt, solange er gewaehlt ist. */
@Composable
private fun Wahl(
    beschriftung: String,
    gewaehlt: Boolean,
    modifier: Modifier = Modifier,
    tat: () -> Unit,
) {
    Box(
        modifier
            .background(if (gewaehlt) Akzent else Flaeche, RoundedCornerShape(8.dp))
            .then(if (gewaehlt) Modifier else Modifier.border(Rahmen, 8.dp))
            .clickable { tat() }
            .padding(vertical = 9.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            beschriftung,
            fontSize = 12.sp,
            fontWeight = if (gewaehlt) FontWeight.SemiBold else FontWeight.Normal,
            color = if (gewaehlt) Grund else Gedaempft,
            maxLines = 1,
        )
    }
}
