plugins {
  id("com.android.application")
  id("org.jetbrains.kotlin.plugin.compose")
}

android {
  namespace = "de.esstracker.android"
  compileSdk = 37

  defaultConfig {
    applicationId = "de.esstracker.android"
    minSdk = 26          // Oreo: darunter fehlt java.time, das hier ueberall steckt
    targetSdk = 37
    versionCode = 1
    versionName = "1.0"
  }

  buildFeatures { compose = true }

  compileOptions {
    sourceCompatibility = JavaVersion.VERSION_17
    targetCompatibility = JavaVersion.VERSION_17
  }

  buildTypes {
    debug { isMinifyEnabled = false }
    release { isMinifyEnabled = false }
  }
}

dependencies {
  val compose = platform("androidx.compose:compose-bom:2025.09.00")
  implementation(compose)
  implementation("androidx.compose.ui:ui")
  implementation("androidx.compose.ui:ui-tooling-preview")
  implementation("androidx.compose.material3:material3")
  implementation("androidx.compose.material:material-icons-extended")
  implementation("androidx.activity:activity-compose:1.9.3")
  implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
  implementation("androidx.core:core-ktx:1.15.0")

  // Die Kamera und der Strichcode-Leser. Das ML-Kit-Modell steckt im APK und
  // braucht weder Play-Dienste noch Netz — Scannen muss auch im Keller des
  // Supermarkts gehen.
  implementation("androidx.camera:camera-core:1.4.1")
  implementation("androidx.camera:camera-camera2:1.4.1")
  implementation("androidx.camera:camera-lifecycle:1.4.1")
  implementation("androidx.camera:camera-view:1.4.1")
  implementation("com.google.mlkit:barcode-scanning:17.3.0")

  testImplementation("junit:junit:4.13.2")
}
