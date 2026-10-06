package de.esstracker.android

import android.Manifest
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.QrCodeScanner
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Der Erfassen-Bildschirm.
 *
 * Zwei Wege zu einem Produkt: den Strichcode scannen oder den Namen suchen.
 * Der Scanner steht oben, weil er der schnellere ist — bei allem Verpackten
 * ist er ein Handgriff, wo die Suche ein Dutzend Tastendruecke waere.
 *
 * Ist ein Produkt gewaehlt, tritt der Mengen-Bildschirm davor. Er ist der
 * einzige Ort, an dem in dieser App getippt werden muss.
 */
@Composable
fun ErfassenBildschirm(zustand: Zustand) {
    val gewaehlt = zustand.gewaehlt
    if (gewaehlt != null) {
        MengeBildschirm(zustand, gewaehlt)
        return
    }
    if (zustand.selbst) {
        SelbstBildschirm(zustand)
        return
    }

    val zusammenhang = LocalContext.current
    var darfKamera by remember { mutableStateOf(darfKamera(zusammenhang)) }
    val frage = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { erlaubt ->
        darfKamera = erlaubt
        if (!erlaubt) {
            zustand.sage("Ohne Kamera geht nur die Suche nach dem Namen.")
        }
    }

    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item {
            Karte {
                KartenKopf("Strichcode", if (darfKamera) "hochkant auf den Code halten" else null)
                if (darfKamera) {
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .aspectRatio(1.35f)
                            .clip(RoundedCornerShape(10.dp))
                            .background(Stufe0)
                    ) {
                        Sucher(Modifier.fillMaxSize()) { code -> zustand.sucheBarcode(code) }
                        // Der Rahmen ueber der Vorschau sagt, wo der Code
                        // hingehoert. Ohne ihn haelt man das Handy zu weit weg.
                        Box(
                            Modifier
                                .align(Alignment.Center)
                                .fillMaxWidth(0.72f)
                                .aspectRatio(2.2f)
                                .border(Akzent, 8.dp)
                        )
                        if (zustand.suchtGerade) {
                            Box(
                                Modifier.fillMaxSize().background(Grund.copy(alpha = 0.6f)),
                                contentAlignment = Alignment.Center,
                            ) { CircularProgressIndicator(color = Akzent, strokeWidth = 2.dp) }
                        }
                    }
                } else {
                    Leerzeile("Die Kamera liest den Strichcode und findet das Produkt in einem Griff.")
                    Luft(4)
                    Knopf("Kamera erlauben", haupt = true) {
                        frage.launch(Manifest.permission.CAMERA)
                    }
                }
            }
        }

        item { SuchKarte(zustand) }

        // Nach einer Suche, die nichts gebracht hat, steht das Angebot oben —
        // direkt dort, wo gerade die Enttaeuschung sitzt. Sonst steht es
        // unauffaellig unter der Trefferliste, damit es den Normalfall nicht
        // verdraengt: die Datenbank kennt die meisten Sachen.
        if (zustand.ohneTreffer) {
            item { SelbstKarte(zustand, auffaellig = true) }
        }

        if (zustand.treffer.isNotEmpty()) {
            // Eindeutig machen, bevor es Schluessel werden: dasselbe Produkt
            // kann in der eigenen Datenbank und in Open Food Facts stehen, und
            // ein doppelter Schluessel bringt die Liste zum Absturz.
            val liste = zustand.treffer.distinctBy { it.quelle + "|" + it.barcode + "|" + it.name }
            items(liste, key = { it.quelle + "|" + it.barcode + "|" + it.name }) { treffer ->
                Trefferzeile(treffer) { zustand.gewaehlt = treffer }
            }
        }

        if (!zustand.ohneTreffer) {
            item { SelbstKarte(zustand, auffaellig = false) }
        }
    }
}

/**
 * Der Weg an der Datenbank vorbei.
 *
 * Er ist immer da, aber nicht immer gleich laut: nach einer Suche ohne
 * Treffer steht er oben und als Hauptknopf, sonst unten und leise. Selbst
 * Gekochtes hat keinen Strichcode, und ohne diesen Weg fiele es aus dem
 * Tagebuch heraus.
 */
@Composable
private fun SelbstKarte(zustand: Zustand, auffaellig: Boolean) {
    Karte {
        KartenKopf(
            if (auffaellig) "Nichts gefunden" else "Nicht in der Datenbank?",
            if (auffaellig) null else "selbst gekocht, Kantine, Bäcker",
        )
        Text(
            if (auffaellig) {
                "Trag es selbst ein — Name und Menge genügen. Die Nährwerte " +
                    "schätzt Claude am Rechner und merkt sie sich für das nächste Mal."
            } else {
                "Selbst Gekochtes hat keinen Strichcode. Name und Menge genügen, " +
                    "die Nährwerte schätzt Claude am Rechner."
            },
            fontSize = 12.5.sp, color = Gedaempft,
        )
        Luft(12)
        Knopf("Selbst eintragen", haupt = auffaellig, modifier = Modifier.fillMaxWidth()) {
            zustand.selbst = true
        }
    }
}

@Composable
private fun SuchKarte(zustand: Zustand) {
    val tastatur = LocalSoftwareKeyboardController.current
    Karte {
        KartenKopf("Nach Namen suchen", "Open Food Facts")
        Row(verticalAlignment = Alignment.CenterVertically) {
            Feld(
                wert = zustand.suchtext,
                aufWert = { zustand.suchtext = it },
                platzhalter = "Haferflocken, Vollmilch, …",
                modifier = Modifier.weight(1f),
                tastatur = KeyboardType.Text,
                aufFertig = { zustand.sucheProdukt() },
            )
            Breite(8)
            if (zustand.suchtGerade) {
                CircularProgressIndicator(
                    color = Akzent, strokeWidth = 2.dp, modifier = Modifier.size(20.dp)
                )
            } else {
                Knopf("Suchen", haupt = true) {
                    tastatur?.hide()
                    zustand.sucheProdukt()
                }
            }
        }
    }
}

@Composable
private fun Trefferzeile(treffer: Datenbank.Treffer, tat: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Flaeche, RoundedCornerShape(10.dp))
            .border(Rahmen, 10.dp)
            .clickable { tat() }
            .padding(horizontal = 14.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                treffer.name, fontSize = 14.sp, fontWeight = FontWeight.Medium, color = Tinte,
                maxLines = 2, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
            )
            Text(
                buildString {
                    if (treffer.quelle == "eigen") append("eigene Datenbank · geschätzt")
                    if (treffer.marke.isNotEmpty()) {
                        if (isNotEmpty()) append(" · ")
                        append(treffer.marke)
                    }
                    if (treffer.menge.isNotEmpty()) {
                        if (isNotEmpty()) append(" · ")
                        append(treffer.menge)
                    }
                },
                fontSize = 11.5.sp, color = Leise, maxLines = 1,
                overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
            )
        }
        Breite(10)
        Column(horizontalAlignment = Alignment.End) {
            Text(
                "${treffer.je100.kcal.toInt()}",
                fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Tinte, fontFamily = Mono,
            )
            Text("kcal/100 ${treffer.einheit}", fontSize = 10.5.sp, color = Leise)
        }
    }
}

/**
 * Ein Eingabefeld in den Kanten der Tasklist: dunkler Grund, duenner Rahmen,
 * heller Rahmen im Fokus.
 */
@Composable
fun Feld(
    wert: String,
    aufWert: (String) -> Unit,
    platzhalter: String,
    modifier: Modifier = Modifier,
    tastatur: KeyboardType = KeyboardType.Text,
    /**
     * Ob die Tastatur „weiter" statt „fertig" anbietet.
     *
     * In einem Formular mit sieben Feldern ist „fertig" nach jedem Feld der
     * falsche Vorschlag: es klappt die Tastatur zu, die man gleich wieder
     * braucht. „Weiter" springt statt dessen ins naechste Feld.
     */
    weiter: Boolean = false,
    aufFertig: () -> Unit = {},
) {
    // Das Haekchen auf der Tastatur muss die Tastatur schliessen, und zwar
    // hier und nicht bei jedem Aufrufer.
    //
    // Compose schliesst sie bei ImeAction.Done von allein — aber nur, solange
    // man `keyboardActions` in Ruhe laesst. Wer dort ein eigenes `onDone`
    // eintraegt, ersetzt das Standardverhalten vollstaendig; ein Feld ohne
    // eigenes `aufFertig` bekam so ein leeres Lambda, und das Haekchen tat
    // buchstaeblich nichts. Auf dem Mengen-Bildschirm blieb die Tastatur
    // dann ueber dem Eintragen-Knopf stehen.
    val tastaturSteuerung = LocalSoftwareKeyboardController.current
    val fokus = LocalFocusManager.current

    OutlinedTextField(
        value = wert,
        onValueChange = aufWert,
        placeholder = { Text(platzhalter, fontSize = 13.5.sp, color = Leise) },
        singleLine = true,
        textStyle = androidx.compose.ui.text.TextStyle(
            fontFamily = if (tastatur == KeyboardType.Number) Mono else Outfit,
            fontSize = 14.sp,
            color = Tinte,
        ),
        keyboardOptions = KeyboardOptions(
            keyboardType = tastatur,
            imeAction = if (weiter) ImeAction.Next else ImeAction.Done,
        ),
        keyboardActions = KeyboardActions(
            onNext = { fokus.moveFocus(FocusDirection.Down) },
            onDone = {
                tastaturSteuerung?.hide()
                fokus.clearFocus()
                aufFertig()
            },
        ),
        shape = RoundedCornerShape(8.dp),
        colors = OutlinedTextFieldDefaults.colors(
            focusedContainerColor = Grund,
            unfocusedContainerColor = Grund,
            focusedBorderColor = Akzent,
            unfocusedBorderColor = Rahmen,
            cursorColor = Akzent,
        ),
        modifier = modifier,
    )
}
