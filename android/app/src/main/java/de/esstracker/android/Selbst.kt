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
 * Selbst eintragen — fuer alles, was Open Food Facts nicht kennt.
 *
 * Das ist kein Randfall: selbst Gekochtes hat keinen Strichcode, die Beilage
 * beim Griechen steht in keiner Datenbank, und regionale Baecker sind dort
 * selten vollstaendig. Ohne diesen Weg bliebe von einem Tag genau das uebrig,
 * was aus einer Packung kam — und das waere ein schiefes Bild.
 *
 * Pflicht ist nur der Name; die Menge folgt auf dem naechsten Bildschirm. Die
 * Kilokalorien waren frueher auch Pflicht — mit dem Ergebnis, dass im Tagebuch
 * „Kulturheidelbeeren, 1 kcal je 100 g“ stand, weil das Feld nicht leer
 * bleiben durfte. Eine erfundene Zahl ist schlechter als eine fehlende.
 *
 * Bleibt das Feld leer, geht der Eintrag als `quelle = "offen"` an den Rechner.
 * Dort schaetzt Claude die Naehrwerte, und der Eintrag kommt mit ihnen
 * zurueck; das Lebensmittel steht danach in der eigenen Datenbank und muss nie
 * wieder geschaetzt werden. Wer die Zahlen von der Packung abliest, traegt sie
 * ein — dann ist es `quelle = "hand"`, und was dasteht, bleibt stehen.
 */
@Composable
fun SelbstBildschirm(zustand: Zustand) {
    val vorlage = zustand.vorlage
    val v = vorlage?.je100

    var name by remember { mutableStateOf(vorlage?.name ?: "") }
    var marke by remember { mutableStateOf(vorlage?.marke ?: "") }
    var einheit by remember { mutableStateOf(vorlage?.einheit ?: "g") }
    var kcal by remember { mutableStateOf(v?.kcal.alsFeld()) }

    var fett by remember { mutableStateOf(v?.fett.alsFeld()) }
    var gesaettigt by remember { mutableStateOf(v?.gesaettigt.alsFeld()) }
    var kohlenhydrate by remember { mutableStateOf(v?.kohlenhydrate.alsFeld()) }
    var zucker by remember { mutableStateOf(v?.zucker.alsFeld()) }
    var eiweiss by remember { mutableStateOf(v?.eiweiss.alsFeld()) }
    var ballaststoffe by remember { mutableStateOf(v?.ballaststoffe.alsFeld()) }
    var salz by remember { mutableStateOf(v?.salz.alsFeld()) }

    // Aufgeklappt, wenn die Vorlage schon etwas mitbringt — sonst stuenden
    // die Zahlen aus Open Food Facts unsichtbar in einem zugeklappten Block.
    var mehr by remember {
        mutableStateOf(listOf(fett, gesaettigt, kohlenhydrate, zucker, eiweiss, ballaststoffe, salz).any { it.isNotEmpty() })
    }

    val kcalZahl = kcal.zahl()
    val gueltig = name.isNotBlank()

    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item {
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = {
                    zustand.selbst = false
                    zustand.vorlage = null
                }) {
                    Icon(
                        Icons.Outlined.ArrowBack, contentDescription = "Zurück",
                        tint = Gedaempft, modifier = Modifier.size(20.dp),
                    )
                }
                Text("Selbst eintragen", fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = Tinte)
            }
        }

        item {
            Karte {
                KartenKopf("Was war das?", vorlage?.barcode?.takeIf { it.isNotEmpty() })
                Feld(
                    wert = name,
                    aufWert = { name = it },
                    platzhalter = "z. B. Gulasch, selbst gekocht",
                    modifier = Modifier.fillMaxWidth(),
                    weiter = true,
                )
                Luft(10)
                Feld(
                    wert = marke,
                    aufWert = { marke = it },
                    platzhalter = "Marke oder Ort (freiwillig)",
                    modifier = Modifier.fillMaxWidth(),
                    weiter = true,
                )
            }
        }

        item {
            Karte {
                KartenKopf("Energie", "je 100 $einheit · freiwillig")
                Feld(
                    wert = kcal,
                    aufWert = { kcal = it.nurZahl() },
                    platzhalter = "kcal je 100 $einheit",
                    modifier = Modifier.fillMaxWidth(),
                    tastatur = KeyboardType.Number,
                )
                Luft(8)
                Text(
                    if (kcalZahl > 0) {
                        "Steht es auf der Packung, ist das die beste Zahl."
                    } else {
                        "Weißt du es nicht, lass es leer: Claude schätzt die Nährwerte am " +
                            "Rechner und merkt sie sich für das nächste Mal."
                    },
                    fontSize = 11.5.sp, color = Leise,
                )
                Luft(12)
                // Gemessen wird in Gramm oder Millilitern. Bei einer Suppe
                // oder einem Saft ist das der Unterschied zwischen einer
                // brauchbaren und einer sinnlosen Zahl.
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (e in listOf("g", "ml")) {
                        Umschalter(
                            if (e == "g") "Gramm" else "Milliliter",
                            gewaehlt = einheit == e,
                            modifier = Modifier.weight(1f),
                        ) { einheit = e }
                    }
                }
            }
        }

        item {
            Karte {
                Row(
                    Modifier.fillMaxWidth().clickable { mehr = !mehr },
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "Weitere Nährwerte",
                        fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                    )
                    Text(
                        if (mehr) "einklappen" else "freiwillig",
                        fontSize = 12.sp, color = Leise,
                    )
                }

                if (mehr) {
                    Luft(12)
                    Text(
                        "Was du nicht weißt, lässt du leer. Eine geschätzte Zahl steht " +
                            "auf dem Rechner später neben nachgeschlagenen und sieht " +
                            "genauso sicher aus.",
                        fontSize = 11.5.sp, color = Leise,
                    )
                    Luft(12)
                    Naehrfeld("Fett", fett, einheit) { fett = it }
                    Naehrfeld("davon gesättigt", gesaettigt, einheit) { gesaettigt = it }
                    Naehrfeld("Kohlenhydrate", kohlenhydrate, einheit) { kohlenhydrate = it }
                    Naehrfeld("davon Zucker", zucker, einheit) { zucker = it }
                    Naehrfeld("Ballaststoffe", ballaststoffe, einheit) { ballaststoffe = it }
                    Naehrfeld("Eiweiß", eiweiss, einheit) { eiweiss = it }
                    Naehrfeld("Salz", salz, einheit, letztes = true) { salz = it }
                }
            }
        }

        item {
            Knopf(
                if (name.isBlank()) "Name fehlt" else "Weiter zur Menge",
                haupt = true,
                an = gueltig,
                modifier = Modifier.fillMaxWidth(),
            ) {
                zustand.gewaehlt = Datenbank.Treffer(
                    barcode = vorlage?.barcode ?: "",
                    name = name.trim(),
                    marke = marke.trim(),
                    menge = "",
                    einheit = einheit,
                    quelle = if (kcalZahl > 0) "hand" else "offen",
                    je100 = Naehrwerte(
                        kcal = kcalZahl,
                        // Kilojoule wird gerechnet, nicht gefragt: der Faktor
                        // ist fest, und ein Feld mehr waere ein Feld, das
                        // niemand ausfuellt.
                        kj = kcalZahl * 4.184,
                        fett = fett.zahl(),
                        gesaettigt = gesaettigt.zahl(),
                        kohlenhydrate = kohlenhydrate.zahl(),
                        zucker = zucker.zahl(),
                        ballaststoffe = ballaststoffe.zahl(),
                        eiweiss = eiweiss.zahl(),
                        salz = salz.zahl(),
                    ),
                )
                zustand.selbst = false
                zustand.vorlage = null
            }
        }
    }
}

/** Eine Zeile im Nährwert-Block: Bezeichnung links, Feld rechts. */
@Composable
private fun Naehrfeld(
    bezeichnung: String,
    wert: String,
    einheit: String,
    letztes: Boolean = false,
    aufWert: (String) -> Unit,
) {
    Row(
        Modifier.fillMaxWidth().padding(bottom = if (letztes) 0.dp else 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(bezeichnung, fontSize = 13.sp, color = Gedaempft, modifier = Modifier.weight(1f))
        Breite(10)
        Feld(
            wert = wert,
            aufWert = { aufWert(it.nurZahl()) },
            platzhalter = "g je 100 $einheit",
            modifier = Modifier.weight(1f),
            tastatur = KeyboardType.Number,
            weiter = !letztes,
        )
    }
}

/** Ein Knopf, der an bleibt, solange er gewaehlt ist. */
@Composable
private fun Umschalter(
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
            fontSize = 12.5.sp,
            fontWeight = if (gewaehlt) FontWeight.SemiBold else FontWeight.Normal,
            color = if (gewaehlt) Grund else Gedaempft,
            maxLines = 1,
        )
    }
}

/**
 * Laesst nur durch, was in ein Naehrwertfeld gehoert.
 *
 * Komma und Punkt beide: auf einer deutschen Tastatur tippt man 12,5, die
 * Zahlentastatur bietet aber oft nur den Punkt an.
 */
private fun String.nurZahl(): String = filter { it.isDigit() || it == ',' || it == '.' }.take(7)

/** Schreibt eine Zahl aus der Vorlage ins Feld — eine Null bleibt ein leeres Feld. */
private fun Double?.alsFeld(): String = when {
    this == null || this <= 0.0 -> ""
    this % 1.0 == 0.0 -> toLong().toString()
    else -> toString().replace('.', ',')
}

/** Liest ein Feld als Zahl. Leer und Unsinn ergeben null — nicht einen Fehler. */
private fun String.zahl(): Double = replace(',', '.').toDoubleOrNull()?.takeIf { it >= 0 } ?: 0.0
