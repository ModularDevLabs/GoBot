#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${MODBOT_TOKEN:-}" || -z "${MODBOT_ADMIN_PASS:-}" ]]; then
  echo "Missing MODBOT_TOKEN or MODBOT_ADMIN_PASS."
  echo "Usage: MODBOT_TOKEN=... MODBOT_ADMIN_PASS=... ./run.sh"
  exit 1
fi

DB_PATH="${MODBOT_DB:-modbot.sqlite}"
BIND_ADDR="${MODBOT_BIND:-127.0.0.1:8080}"

exec ./modbot --db "$DB_PATH" --bind "$BIND_ADDR"
