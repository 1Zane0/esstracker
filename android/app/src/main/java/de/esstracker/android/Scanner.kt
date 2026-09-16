package de.esstracker.android

import android.annotation.SuppressLint
import android.content.Context
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.viewinterop.AndroidView
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.common.InputImage
import java.util.concurrent.Executors

/**
 * Der Strichcode-Leser.
 *
 * Das Modell steckt im APK: Scannen geht ohne Netz und ohne Play-Dienste, und
 * das ist keine Nebensache — im Supermarkt ist oft beides nicht da.
 *
 * Gesucht wird nur nach den Formaten, die auf Lebensmitteln stehen. Jedes
 * Format mehr kostet Rechenzeit je Bild und bringt hier nichts: einen QR-Code
 * findet man auf keiner Packung Haferflocken.
 */
@SuppressLint("UnsafeOptInUsageError")
@Composable
fun Sucher(
    modifier: Modifier = Modifier,
    aufCode: (String) -> Unit,
) {
    val zusammenhang = LocalContext.current
    val lebenslauf = LocalLifecycleOwner.current
    val werkbank = remember { Executors.newSingleThreadExecutor() }

    val leser = remember {
        BarcodeScanning.getClient(
            BarcodeScannerOptions.Builder()
                .setBarcodeFormats(
                    Barcode.FORMAT_EAN_13,   // der Normalfall in Europa
                    Barcode.FORMAT_EAN_8,    // kleine Packungen
                    Barcode.FORMAT_UPC_A,    // Einfuhr aus Nordamerika
                    Barcode.FORMAT_UPC_E,
                    Barcode.FORMAT_ITF,      // Umkartons, selten auf Endverpackungen
                )
                .build()
        )
    }

    // Ein einmal gelesener Code darf nicht dreissigmal je Sekunde erneut
    // gemeldet werden, solange die Packung vor der Linse liegt.
    val zuletzt = remember { arrayOfNulls<String>(1) }

    DisposableEffect(Unit) {
        onDispose {
            werkbank.shutdown()
            leser.close()
        }
    }

    AndroidView(
        modifier = modifier,
        factory = { ctx ->
            val sicht = PreviewView(ctx).apply {
                scaleType = PreviewView.ScaleType.FILL_CENTER
            }
            val zukunft = ProcessCameraProvider.getInstance(ctx)
            zukunft.addListener({
                val anbieter = zukunft.get()
                val vorschau = Preview.Builder().build().also {
                    it.surfaceProvider = sicht.surfaceProvider
                }
                val auswertung = ImageAnalysis.Builder()
                    // Bilder fallen lassen statt anstauen: ein Strichcode, der
                    // vor zwei Sekunden im Bild war, interessiert nicht mehr.
                    .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                    .build()

                auswertung.setAnalyzer(werkbank) { bild: ImageProxy ->
                    val medium = bild.image
                    if (medium == null) {
                        bild.close()
                        return@setAnalyzer
                    }
                    val eingabe = InputImage.fromMediaImage(medium, bild.imageInfo.rotationDegrees)
                    leser.process(eingabe)
                        .addOnSuccessListener { codes ->
                            val code = codes.firstNotNullOfOrNull { it.rawValue }
                            if (code != null && code != zuletzt[0]) {
                                zuletzt[0] = code
                                aufCode(code)
                            }
                        }
                        .addOnCompleteListener { bild.close() }
                }

                try {
                    anbieter.unbindAll()
                    anbieter.bindToLifecycle(
                        lebenslauf, CameraSelector.DEFAULT_BACK_CAMERA, vorschau, auswertung
                    )
                } catch (fehler: Exception) {
                    // Kein Bild. Der Bildschirm daneben bietet die Textsuche an —
                    // eine Fehlermeldung an dieser Stelle wuerde nur die
                    // Vorschau verdecken, die ohnehin schwarz bleibt.
                }
            }, androidx.core.content.ContextCompat.getMainExecutor(ctx))
            sicht
        }
    )
}

/** Ob diese App gerade die Kamera benutzen darf. */
fun darfKamera(zusammenhang: Context): Boolean =
    androidx.core.content.ContextCompat.checkSelfPermission(
        zusammenhang, android.Manifest.permission.CAMERA
    ) == android.content.pm.PackageManager.PERMISSION_GRANTED
