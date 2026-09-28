#!/bin/sh
# Vyrenderuje ikonu a spouštěcí obrázek z SVG v assets/brand/ do PNG, která
# čte app.json. Po změně kresby spustit znovu a pak `expo prebuild --clean`.
#
# Potřebuje rsvg-convert (macOS: `brew install librsvg`).
set -eu

cd "$(dirname "$0")/../assets"

if ! command -v rsvg-convert >/dev/null 2>&1; then
  echo "Chybí rsvg-convert – nainstalujte ho: brew install librsvg" >&2
  exit 1
fi

render() {
  rsvg-convert -w "$3" -h "$3" "brand/$1" -o "$2"
  echo "  $2 (${3}px)"
}

echo "Generuji ikonu a splash z assets/brand/:"
render icon.svg icon.png 1024
render icon-background.svg android-icon-background.png 1024
render foreground.svg android-icon-foreground.png 1024
render foreground.svg android-icon-monochrome.png 1024
render splash.svg splash-icon.png 1024
render icon.svg favicon.png 48
