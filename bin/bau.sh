#!/usr/bin/env bash
# Baut das Programm. Ohne Argument mit Fenster, mit --ohne-fenster ohne.
#
# Der PKG_CONFIG_PATH zeigt auf bin/pkgconfig: dort liegt die Bruecke von
# webkit2gtk-4.0 (was webview_go verlangt) auf 4.1 (was Arch liefert).
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "${1:-}" == "--ohne-fenster" ]]; then
  echo "→ baue ohne Fenster (nur Dienst)"
  go build -tags ohne_fenster -o esstracker ./cmd/esstracker
else
  export PKG_CONFIG_PATH="$PWD/bin/pkgconfig:${PKG_CONFIG_PATH:-}"
  if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1; then
    echo "Es fehlen gtk3 und/oder webkit2gtk-4.1 — ohne die gibt es kein Fenster." >&2
    echo "  sudo pacman -S gtk3 webkit2gtk-4.1" >&2
    echo "Oder ohne Fenster bauen:  bin/bau.sh --ohne-fenster" >&2
    exit 1
  fi
  go build -o esstracker ./cmd/esstracker
fi

echo "→ $(pwd)/esstracker ($(du -h esstracker | cut -f1))"
