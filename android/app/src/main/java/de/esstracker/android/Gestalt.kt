package de.esstracker.android

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/**
 * Die Gestalt der App — dieselben Werte wie in `stil.css` auf dem Rechner.
 *
 * Das ist der ganze Zweck dieser Datei: wer die App neben dem Fenster auf dem
 * Rechner sieht, soll ein Werkzeug sehen und nicht zwei. Deshalb sind die
 * Farben hier keine Material-Vorgaben und keine Ableitung aus einer
 * Grundfarbe, sondern dieselben Hexwerte, Zeichen fuer Zeichen.
 */

// ---------- Farben ----------

val Grund = Color(0xFF0A0A0B)
val Flaeche = Color(0xFF121214)
val FlaecheHoch = Color(0xFF1A1A1E)
val Rahmen = Color(0xFF26262B)
val RahmenHell = Color(0xFF35353D)
val Tinte = Color(0xFFEDEDF0)
val Gedaempft = Color(0xFFA0A0AB)
val Leise = Color(0xFF6E6E7A)
val Akzent = Color(0xFFFAFAFA)
val AkzentTief = Color(0xFFD8D8DC)

// Die fuenf Stufen laufen ueber Helligkeit, nicht ueber Farbe — wie im
// Jahresraster auf dem Rechner.
val Stufe0 = Color(0xFF17171A)
val Stufe1 = Color(0xFF33333B)
val Stufe2 = Color(0xFF5A5A66)
val Stufe3 = Color(0xFF9C9CA8)
val Stufe4 = Color(0xFFFAFAFA)

/**
 * Die Farbtabelle fuer Material 3.
 *
 * Material will Rollen (primary, surface, …), die Tasklist kennt Flaechen und
 * Helligkeiten. Die Zuordnung steht hier einmal, damit sie nicht in jedem
 * Bildschirm neu erfunden wird: `primary` ist das helle Weiss, auf dem Schwarz
 * steht, `surface` die Kartenflaeche, `outline` die Kante.
 */
val Farben = darkColorScheme(
    primary = Akzent,
    onPrimary = Grund,
    primaryContainer = FlaecheHoch,
    onPrimaryContainer = Tinte,
    secondary = Gedaempft,
    onSecondary = Grund,
    background = Grund,
    onBackground = Tinte,
    surface = Flaeche,
    onSurface = Tinte,
    surfaceVariant = FlaecheHoch,
    onSurfaceVariant = Gedaempft,
    outline = Rahmen,
    outlineVariant = RahmenHell,
    error = Stufe4,
    onError = Grund,
)

// ---------- Schrift ----------

// Dieselbe Outfit, die auf dem Rechner aus der Datei nebenan kommt. Sie liegt
// als variable Schrift vor; Android waehlt daraus die Schnitte.
val Outfit = FontFamily(
    Font(R.font.outfit, FontWeight.Normal),
    Font(R.font.outfit, FontWeight.Medium),
    Font(R.font.outfit, FontWeight.SemiBold),
    Font(R.font.outfit, FontWeight.Bold),
)

// Messwerte werden monospaced gesetzt: untereinander soll man Kalorien
// ueberschlagen koennen, ohne sie zu lesen.
val Mono = FontFamily.Monospace

val Schriften = Typography().let { vorgabe ->
    Typography(
        displayLarge = vorgabe.displayLarge.copy(fontFamily = Outfit, letterSpacing = (-1).sp),
        headlineSmall = vorgabe.headlineSmall.copy(fontFamily = Outfit, fontWeight = FontWeight.SemiBold),
        titleLarge = vorgabe.titleLarge.copy(fontFamily = Outfit, fontWeight = FontWeight.SemiBold, fontSize = 19.sp),
        titleMedium = vorgabe.titleMedium.copy(fontFamily = Outfit, fontWeight = FontWeight.SemiBold, fontSize = 15.sp),
        titleSmall = vorgabe.titleSmall.copy(fontFamily = Outfit, fontWeight = FontWeight.SemiBold, fontSize = 13.sp),
        bodyLarge = vorgabe.bodyLarge.copy(fontFamily = Outfit, fontSize = 15.sp),
        bodyMedium = vorgabe.bodyMedium.copy(fontFamily = Outfit, fontSize = 13.5.sp),
        bodySmall = vorgabe.bodySmall.copy(fontFamily = Outfit, fontSize = 12.sp, color = Leise),
        labelLarge = vorgabe.labelLarge.copy(fontFamily = Outfit, fontWeight = FontWeight.Medium),
        labelMedium = vorgabe.labelMedium.copy(fontFamily = Outfit, fontSize = 11.5.sp),
        labelSmall = vorgabe.labelSmall.copy(fontFamily = Outfit, fontSize = 11.sp, letterSpacing = 0.5.sp),
    )
}

/** Der Stil fuer Zahlen: monospaced, damit Spalten stehen. */
val Messwert = TextStyle(fontFamily = Mono, fontSize = 12.sp, color = Leise)

@Composable
fun EsstrackerGestalt(inhalt: @Composable () -> Unit) {
    MaterialTheme(colorScheme = Farben, typography = Schriften, content = inhalt)
}
