// AGP 9.3.2 bringt Kotlin selbst mit. Ein zusaetzliches `kotlin("android")`
// gehoert deshalb ausdruecklich NICHT hierher — es scheitert mit
// „Cannot add extension with name 'kotlin'". Das Compose-Plugin dagegen
// braucht es, sobald `buildFeatures { compose = true }` gesetzt ist.
plugins {
  id("com.android.application") version "9.3.2" apply false
  id("org.jetbrains.kotlin.plugin.compose") version "2.4.10" apply false
}
