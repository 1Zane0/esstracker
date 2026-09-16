package de.esstracker.android

import androidx.compose.foundation.background
import androidx.compose.foundation.border as androidxBorder
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.CalendarToday
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Snackbar
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Das Geruest: unten drei Reiter, darin der jeweilige Bildschirm.
 *
 * Drei und nicht mehr. Die App hat einen Tracker, und der braucht: die Zahl
 * von heute, einen Weg, etwas einzutragen, und einen Ort fuer den Rechner.
 */
@Composable
fun Geruest(zustand: Zustand) {
    val meldungen = remember { SnackbarHostState() }

    // Beim Aufschlagen nachsehen, ob noch etwas in der Warteschlange liegt.
    // Das ist der haeufigste Fall, in dem etwas nachzuholen ist: der Rechner
    // war beim Eintragen aus und ist jetzt an.
    LaunchedEffect(Unit) {
        zustand.frischAuf()
        zustand.uebertrage(still = true)
    }

    LaunchedEffect(zustand.meldung) {
        zustand.meldung?.let {
            meldungen.showSnackbar(it, duration = SnackbarDuration.Short)
            zustand.sage(null)
        }
    }

    Scaffold(
        containerColor = Grund,
        // Die Einfassungen bleiben: `enableEdgeToEdge` zeichnet bis unter die
        // Statusleiste, und ohne diese Angabe steht die erste Karte hinter der
        // Uhrzeit. Die Leiste unten holt sich ihre eigene Einfassung selbst.
        contentWindowInsets = WindowInsets.systemBars,
        snackbarHost = {
            SnackbarHost(meldungen) { daten ->
                // Die Meldung traegt dieselben Kanten wie die auf dem Rechner:
                // heller Kasten, duenner Rahmen, kein Farbsignal.
                Snackbar(
                    containerColor = FlaecheHoch,
                    contentColor = Tinte,
                    shape = RoundedCornerShape(10.dp),
                    modifier = Modifier.padding(12.dp),
                ) { Text(daten.visuals.message, fontSize = 13.sp) }
            }
        },
        bottomBar = { Leiste(zustand) },
    ) { rand ->
        Box(
            Modifier
                .fillMaxSize()
                .padding(rand)
                // Unter `enableEdgeToEdge` schiebt Android den Inhalt nicht
                // mehr selbst ueber die Tastatur — `adjustResize` im Manifest
                // greift dann nicht mehr. Ohne diese Zeile steht die Tastatur
                // ueber dem Eintragen-Knopf, und der Eintrag laesst sich nicht
                // abschliessen.
                .imePadding()
                .background(Grund)
        ) {
            when (zustand.seite) {
                Seite.Heute -> HeuteBildschirm(zustand)
                Seite.Erfassen -> ErfassenBildschirm(zustand)
                Seite.Einstellungen -> EinstellungenBildschirm(zustand)
            }
        }
    }
}

@Composable
private fun Leiste(zustand: Zustand) {
    NavigationBar(containerColor = Flaeche, tonalElevation = 0.dp) {
        Reiter(zustand, Seite.Heute, Icons.Outlined.CalendarToday, "Heute")
        Reiter(zustand, Seite.Erfassen, Icons.Outlined.Add, "Eintragen")
        Reiter(zustand, Seite.Einstellungen, Icons.Outlined.Settings, "Rechner")
    }
}

@Composable
private fun androidx.compose.foundation.layout.RowScope.Reiter(
    zustand: Zustand,
    seite: Seite,
    symbol: androidx.compose.ui.graphics.vector.ImageVector,
    beschriftung: String,
) {
    NavigationBarItem(
        selected = zustand.seite == seite,
        onClick = { zustand.seite = seite },
        icon = { Icon(symbol, contentDescription = null, Modifier.size(20.dp)) },
        label = { Text(beschriftung, fontSize = 11.sp) },
        colors = NavigationBarItemDefaults.colors(
            selectedIconColor = Grund,
            selectedTextColor = Tinte,
            indicatorColor = Akzent,
            unselectedIconColor = Leise,
            unselectedTextColor = Leise,
        ),
    )
}

// ---------- Bausteine, die mehrere Bildschirme brauchen ----------

/**
 * Eine Karte — dieselbe Flaeche, derselbe Rahmen, dieselbe Rundung wie
 * `.karte` auf dem Rechner.
 */
@Composable
fun Karte(
    modifier: Modifier = Modifier,
    inhalt: @Composable androidx.compose.foundation.layout.ColumnScope.() -> Unit,
) {
    Column(
        modifier
            .fillMaxWidth()
            .background(Flaeche, RoundedCornerShape(12.dp))
            .border(Rahmen, 12.dp)
            .padding(16.dp),
        content = inhalt,
    )
}

/**
 * Der Rahmen einer Karte. Eigene Funktion, damit Breite und Rundung an jeder
 * Kante dieselben sind — so wie `--rahmen` und `--r-karte` auf dem Rechner.
 *
 * Der Name verdeckt absichtlich `Modifier.border` aus der Standardbibliothek:
 * wer hier `border(Rahmen, 10.dp)` schreibt, soll nicht versehentlich eine
 * andere Strichbreite bekommen.
 */
fun Modifier.border(farbe: Color, rundung: Dp): Modifier =
    this.androidxBorder(1.dp, farbe, RoundedCornerShape(rundung))

/** Die Ueberschrift in einer Karte, mit optionalem Zusatz rechts. */
@Composable
fun KartenKopf(titel: String, zusatz: String? = null) {
    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(titel, fontSize = 13.5.sp, fontWeight = FontWeight.SemiBold, color = Tinte)
        zusatz?.let { Text(it, fontSize = 12.sp, color = Leise) }
    }
    Spacer(Modifier.height(12.dp))
}

/**
 * Der Balken, der zeigt, wie weit etwas ist.
 *
 * Er laeuft ueber Helligkeit und nicht ueber Farbe — wie alles hier. Ueber dem
 * Ziel wird er nicht rot, er wird nur voll: die App bewertet nicht, was
 * jemand isst, sie zaehlt es.
 */
@Composable
fun Balken(anteil: Float, hoehe: Int = 4, farbe: Color = Akzent) {
    Box(
        Modifier
            .fillMaxWidth()
            .height(hoehe.dp)
            .background(FlaecheHoch, RoundedCornerShape(hoehe.dp))
    ) {
        Box(
            Modifier
                .fillMaxWidth(anteil.coerceIn(0f, 1f))
                .height(hoehe.dp)
                .background(farbe, RoundedCornerShape(hoehe.dp))
        )
    }
}

/** Eine waagerechte Linie in Rahmenfarbe. */
@Composable
fun Trennlinie(modifier: Modifier = Modifier) {
    Box(
        modifier
            .fillMaxWidth()
            .height(1.dp)
            .background(Rahmen)
    )
}

/** Ein Satz fuer den Fall, dass eine Liste leer ist. */
@Composable
fun Leerzeile(text: String) {
    Text(text, fontSize = 13.sp, color = Leise, modifier = Modifier.padding(vertical = 10.dp))
}

/** Abstand, als eigene Funktion nur wegen der Lesbarkeit weiter unten. */
@Composable
fun Luft(hoehe: Int) = Spacer(Modifier.height(hoehe.dp))

@Composable
fun Breite(breite: Int) = Spacer(Modifier.width(breite.dp))
