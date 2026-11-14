#!/bin/sh
# Spouštěč mediateru v kontejneru.
#
# UMASK – nepovinné. Produkce na NFS potřebuje 0000 (soubory 777, aby je mohli
# upravovat i ostatní klienti NAS); bez proměnné platí výchozí umask image.
set -eu

if [ -n "${UMASK:-}" ]; then
    umask "$UMASK"
fi

exec /app/mediater "$@"
