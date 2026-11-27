#!/bin/sh
# Spouštěč mediateru v kontejneru.
#
# UMASK – nepovinné; bez proměnné platí výchozí umask image (022). Produkce na
# NFS používá 0000. Umask jen ubírá práva, která aplikace při vytváření žádá:
# složky vznikají 755, soubory 644 a databáze 600 (nastavuje ji kód), takže
# ani s 0000 nic nebude zapisovatelné pro všechny.
set -eu

if [ -n "${UMASK:-}" ]; then
    umask "$UMASK"
fi

exec /app/mediater "$@"
