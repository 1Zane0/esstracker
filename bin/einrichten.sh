#!/usr/bin/env bash
# Baut, legt das Binary nach ~/.local/bin und traegt den Starter ein.
#
# Der Name des Binaries ist die ganze dmenu-Anbindung: dmenu_run listet die
# ausfuehrbaren Dateien im PATH, und ~/.local/bin steht dort.
set -euo pipefail
cd "$(dirname "$0")/.."

bin/bau.sh "$@"

ziel="$HOME/.local/bin"
mkdir -p "$ziel"
install -m 755 esstracker "$ziel/esstracker"
echo "→ installiert: $ziel/esstracker"

"$ziel/esstracker" einrichten

if ! command -v esstracker >/dev/null 2>&1; then
  echo
  echo "Hinweis: $ziel liegt nicht im PATH. Zum Aufrufen per Namen:"
  echo '  echo '"'"'export PATH="$HOME/.local/bin:$PATH"'"'"' >> ~/.bashrc'
fi
