#!/usr/bin/env bash
# Baut die Android-App und legt das APK neben dieses Verzeichnis.
#
# Warum hier alles ausdruecklich gesetzt wird: das System-Java ist 26, und das
# Android-Gradle-Plugin nimmt 17 oder 21. Ohne JAVA_HOME bricht der Bau mit
# einer Meldung ab, die den Grund nicht nennt.
#
# Mit --installieren geht das APK anschliessend auf ein angeschlossenes Geraet.
# Haengt mehr als eines dran, muss die Seriennummer dazu:
#   bin/app-bauen.sh --installieren emulator-5554
set -euo pipefail
cd "$(dirname "$0")/../android"

export JAVA_HOME="${JAVA_HOME:-$HOME/.local/share/jdk-21}"
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
gradle="${GRADLE:-$HOME/.local/share/gradle-9.7.1/bin/gradle}"

if [[ ! -x "$gradle" ]]; then
  echo "Gradle nicht gefunden: $gradle" >&2
  echo "Anderen Pfad über GRADLE=… angeben." >&2
  exit 1
fi

"$gradle" :app:assembleDebug --console=plain

apk="app/build/outputs/apk/debug/app-debug.apk"
ziel="../esstracker-$(date +%Y-%m-%d).apk"
cp "$apk" "$ziel"
echo "→ $(cd .. && pwd)/$(basename "$ziel") ($(du -h "$ziel" | cut -f1))"

# Ein gruener Bau beweist nicht, dass die Kotlin-Klassen im Dex stehen.
if command -v apkanalyzer >/dev/null 2>&1; then
  zahl=$(apkanalyzer dex packages "$apk" 2>/dev/null | grep -c '^C d.*de\.esstracker' || true)
  echo "→ $zahl eigene Klassen im Dex"
fi

if [[ "${1:-}" == "--installieren" ]]; then
  geraet=("${2:+-s}" ${2:+"$2"})
  adb "${geraet[@]}" install -r "$apk"
  adb "${geraet[@]}" shell am start -n de.esstracker.android/.Haupt
fi
