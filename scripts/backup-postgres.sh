#!/usr/bin/env bash
# Backup llms Postgres — logical dump + optional data dir tar.
# Usage: LLMS_DATA_DIR=/path/to/data ./scripts/backup-postgres.sh [output-dir]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATA_DIR="${LLMS_DATA_DIR:-$ROOT/data}"
OUT_DIR="${1:-$ROOT/backups}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$OUT_DIR"

COMPOSE_FILE="$ROOT/deploy/podman-compose.yaml"
if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "missing $COMPOSE_FILE" >&2
  exit 1
fi

PG_CONTAINER="$(podman compose -f "$COMPOSE_FILE" ps --format '{{.Names}}' 2>/dev/null | grep -E 'postgres' | head -1 || true)"
DUMP="$OUT_DIR/llms-$STAMP.dump"

if [[ -n "$PG_CONTAINER" ]]; then
  echo "pg_dump via container $PG_CONTAINER -> $DUMP"
  podman exec "$PG_CONTAINER" pg_dump -U llms -d llms -Fc > "$DUMP"
else
  echo "no postgres container; trying host pg_dump on 127.0.0.1:54329" >&2
  pg_dump -h 127.0.0.1 -p 54329 -U llms -d llms -Fc > "$DUMP"
fi

if [[ -d "$DATA_DIR/postgres" ]]; then
  TAR="$OUT_DIR/llms-postgres-data-$STAMP.tgz"
  echo "tar data dir -> $TAR"
  tar -C "$DATA_DIR" -czf "$TAR" postgres
fi

echo "backup done: $DUMP"
