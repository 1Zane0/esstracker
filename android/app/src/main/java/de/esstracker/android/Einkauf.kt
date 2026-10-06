package de.esstracker.android

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.Check
import androidx.compose.material.icons.outlined.ExpandMore
import androidx.compose.material.icons.outlined.Remove
import androidx.compose.material.icons.outlined.ShoppingCart
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.util.Locale

/**
 * Die Einkaufsliste.
 *
 * Der einzige Bildschirm, auf dem etwas steht, das nicht hier entstanden ist:
 * geschrieben wird die Liste am Rechner, wo Claude das Tagebuch kennt. Hier
 * wird sie abgearbeitet — antippen hakt ab, + und − verstellen die Menge,
 * langes Druecken nimmt eine Zeile weg.
 *
 * Preise stehen nicht drauf. Es gibt Mengen, und die kommen aus dem Tagebuch,
 * nicht aus einem Regal.
 */
@Composable
fun EinkaufBildschirm(zustand: Zustand) {
    val liste = zustand.einkaufsListe

    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = androidx.compose.foundation.layout.PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        item(key = "kopf") { Kopf(zustand) }

        if (liste.isEmpty()) {
            item(key = "leer") {
                Karte {
                    Leerzeile(
                        if (zustand.gekoppelt)
                            "Noch keine Liste. Geschrieben wird sie am Rechner — dort im Reiter „Einkauf“."
                        else
                            "Noch kein Rechner gekoppelt. Die Liste kommt von dort."
                    )
                }
            }
        }

        // Nach Abteilung gebuendelt, in der Reihenfolge, die der Rechner
        // vorgibt: so arbeitet man den Laden in einem Gang ab. groupBy haelt
        // die Reihenfolge des Auftretens, und die Liste kommt sortiert an.
        for ((abteilung, zeilen) in liste.groupBy { it.gruppe }) {
            item(key = "abteilung-$abteilung") {
                AbteilungsKopf(abteilung, zeilen.count { !it.erledigt })
            }
            items(zeilen, key = { it.id }) { posten ->
                Postenzeile(posten, zustand)
            }
        }

        item(key = "neu") { NeuerPosten(zustand) }
        item(key = "hinweis") {
            Text(
                "Antippen hakt ab. Lange drücken nimmt einen Posten von der Liste.",
                fontSize = 11.5.sp, color = Leise,
                modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
            )
        }
    }
}

/** Oben steht, wie viel noch zu holen ist — und ob der Rechner davon weiss. */
@Composable
private fun Kopf(zustand: Zustand) {
    val offen = zustand.einkaufOffen
    val gesamt = zustand.einkaufsListe.size

    Karte {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(
                Icons.Outlined.ShoppingCart, contentDescription = null,
                tint = if (offen == 0 && gesamt > 0) Gedaempft else Tinte,
                modifier = Modifier.size(20.dp),
            )
            Breite(12)
            Column(Modifier.weight(1f)) {
                Text(
                    when {
                        gesamt == 0 -> "Einkaufsliste"
                        offen == 0 -> "Alles im Wagen"
                        offen == 1 -> "1 Posten fehlt noch"
                        else -> "$offen Posten fehlen noch"
                    },
                    fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Tinte,
                )
                Text(
                    when {
                        !zustand.gekoppelt -> "Kein Rechner gekoppelt"
                        zustand.einkaufWartet == 1 -> "1 Änderung noch nicht beim Rechner"
                        zustand.einkaufWartet > 1 -> "${zustand.einkaufWartet} Änderungen noch nicht beim Rechner"
                        gesamt == 0 -> "Kommt vom Rechner"
                        else -> "$gesamt Posten · abgeglichen"
                    },
                    fontSize = 11.5.sp, color = Leise,
                )
            }
            if (zustand.gekoppelt) {
                Knopf(
                    if (zustand.gleichtEinkaufAb) "…" else "Abgleichen",
                    an = !zustand.gleichtEinkaufAb,
                ) { zustand.einkaufAbgleichen() }
            }
        }
        if (gesamt > 0) {
            Luft(12)
            Balken((gesamt - offen) / gesamt.toFloat(), hoehe = 4)
        }
    }
}

@Composable
private fun AbteilungsKopf(name: String, offen: Int) {
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
        Text(if (offen > 0) "$offen offen" else "erledigt", fontSize = 11.5.sp, color = Leise)
    }
}

/**
 * Eine Zeile: Haken, Name, Menge.
 *
 * Abgehaktes bleibt stehen und wird blass. Eine Zeile, die beim Antippen
 * verschwindet, nimmt die Moeglichkeit mit, den Haken zurueckzunehmen — und
 * im Laden greift man oefter daneben als am Schreibtisch.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun Postenzeile(posten: EinkaufsPosten, zustand: Zustand) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Flaeche, RoundedCornerShape(10.dp))
            .border(Rahmen, 10.dp)
            .combinedClickable(
                onClick = { zustand.hakeAb(posten) },
                onLongClick = { zustand.postenWeg(posten) },
            )
            .padding(start = 12.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(22.dp)
                .background(if (posten.erledigt) Akzent else Grund, RoundedCornerShape(6.dp))
                .border(if (posten.erledigt) Akzent else RahmenHell, 6.dp),
            contentAlignment = Alignment.Center,
        ) {
            if (posten.erledigt) {
                Icon(Icons.Outlined.Check, contentDescription = null, tint = Grund, modifier = Modifier.size(15.dp))
            }
        }
        Breite(12)

        Column(Modifier.weight(1f)) {
            Text(
                posten.name,
                fontSize = 14.sp,
                fontWeight = if (posten.erledigt) FontWeight.Normal else FontWeight.Medium,
                color = if (posten.erledigt) Leise else Tinte,
                textDecoration = if (posten.erledigt) TextDecoration.LineThrough else null,
                maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
            Text(
                posten.grund.ifEmpty { if (posten.von == "hand") "selbst dazugeschrieben" else posten.gruppe },
                fontSize = 11.5.sp, color = Leise, maxLines = 1, overflow = TextOverflow.Ellipsis,
            )
        }

        Breite(4)
        Stufenknopf(Icons.Outlined.Remove, "weniger") { zustand.postenSchritt(posten, -1) }
        Text(
            zustand.einkauf.mengeText(posten),
            fontSize = 12.5.sp, fontFamily = Mono,
            color = if (posten.erledigt) Leise else Tinte,
            textAlign = TextAlign.Center, maxLines = 1,
            modifier = Modifier.width(74.dp),
        )
        Stufenknopf(Icons.Outlined.Add, "mehr") { zustand.postenSchritt(posten, 1) }
    }
}

@Composable
private fun Stufenknopf(
    symbol: androidx.compose.ui.graphics.vector.ImageVector,
    beschriftung: String,
    tat: () -> Unit,
) {
    Box(
        Modifier
            .size(32.dp)
            .background(FlaecheHoch, RoundedCornerShape(8.dp))
            .clickable(onClickLabel = beschriftung) { tat() },
        contentAlignment = Alignment.Center,
    ) {
        Icon(symbol, contentDescription = beschriftung, tint = Tinte, modifier = Modifier.size(16.dp))
    }
}

/**
 * Was im Laden einfaellt, kommt hier dazu — und beim naechsten Abgleich auch
 * auf die Liste am Rechner.
 */
@Composable
private fun NeuerPosten(zustand: Zustand) {
    var name by remember { mutableStateOf("") }
    var menge by remember { mutableStateOf("") }
    var einheit by remember { mutableStateOf("stueck") }
    var wahlOffen by remember { mutableStateOf(false) }

    val anlegen = {
        if (name.isNotBlank()) {
            zustand.postenAnlegen(name, menge.replace(',', '.').toDoubleOrNull() ?: 0.0, einheit)
            name = ""
            menge = ""
        }
    }

    Karte {
        KartenKopf("Selbst dazuschreiben")
        Feld(name, { name = it }, "Was fehlt noch?", Modifier.fillMaxWidth(), aufFertig = anlegen)
        Luft(8)
        Row(verticalAlignment = Alignment.CenterVertically) {
            Feld(
                menge, { menge = it.filter { z -> z.isDigit() || z == ',' || z == '.' } }, "1",
                Modifier.width(86.dp), tastatur = KeyboardType.Number, aufFertig = anlegen,
            )
            Breite(8)
            Box {
                Row(
                    Modifier
                        .background(FlaecheHoch, RoundedCornerShape(8.dp))
                        .border(Rahmen, 8.dp)
                        .clickable { wahlOffen = true }
                        .padding(horizontal = 12.dp, vertical = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(zustand.einkauf.einheitZu(einheit).eins, fontSize = 13.sp, color = Tinte)
                    Icon(Icons.Outlined.ExpandMore, contentDescription = null, tint = Leise, modifier = Modifier.size(16.dp))
                }
                DropdownMenu(expanded = wahlOffen, onDismissRequest = { wahlOffen = false }) {
                    for (e in zustand.einkauf.einheiten) {
                        DropdownMenuItem(
                            text = { Text(if (e.eins == e.viele) e.eins else "${e.eins} / ${e.viele}", fontSize = 13.sp) },
                            onClick = { einheit = e.name; wahlOffen = false },
                        )
                    }
                }
            }
            Breite(8)
            Knopf("Dazu", haupt = true, an = name.isNotBlank(), modifier = Modifier.weight(1f)) { anlegen() }
        }
    }
}
