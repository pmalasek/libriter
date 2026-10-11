#!/bin/sh
# Vyrenderuje ikony, splash a bannery TV aplikace z assets/brand/ do PNG,
# která čte app.json. Po změně kresby spustit znovu a pak `expo prebuild`.
# Potřebuje jen Node; renderer (resvg) se stáhne do dočasné složky.
set -eu
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
(cd "$tmp" && npm init -y >/dev/null && npm i --silent --no-audit --no-fund @resvg/resvg-js@2.6.2)
node assets/brand/render.mjs "$tmp"
