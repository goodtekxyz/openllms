#!/usr/bin/env bash
# Publish the OSS export tree to github.com/goodtekxyz/openllms.
# Usage:
#   scripts/oss-publish.sh           # sync, check, push
#   scripts/oss-publish.sh --dry-run # sync + check only
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${OSS_EXPORT_DIR:-"$ROOT/.oss-export"}"
REMOTE_URL="${OPENLLMS_REMOTE:-git@github.com:goodtekxyz/openllms.git}"
BRANCH="${OPENLLMS_BRANCH:-main}"
DRY_RUN=0

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help)
      echo "usage: $0 [--dry-run]" >&2
      exit 0
      ;;
    *)
      echo "unknown arg: $arg" >&2
      exit 2
      ;;
  esac
done

cd "$ROOT"
SHA="$(git rev-parse HEAD)"
SHORT="$(git rev-parse --short HEAD)"
MSG="sync from goodtekxyz/llms @ ${SHORT}"

echo "==> oss-sync"
bash "$ROOT/scripts/oss-sync.sh" "$DEST"
echo "==> oss-sync-check"
bash "$ROOT/scripts/oss-sync-check.sh" "$DEST"

# Smoke: OSS default build + a few package tests (no cloud tag).
echo "==> OSS build/test smoke in export"
(
  cd "$DEST"
  go build -o /tmp/openllms-gateway-smoke ./cmd/llms-gateway
  go test ./internal/secrets/file/... ./internal/config/... -count=1
)

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "dry-run OK — skipped push ($DEST @ $SHA)"
  exit 0
fi

WORK="$(mktemp -d "${TMPDIR:-/tmp}/openllms-publish.XXXXXX")"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

echo "==> prepare publish worktree"
rsync -a --delete \
  --exclude='.git/' \
  "$DEST/" "$WORK/"

cd "$WORK"
git init -q
git checkout -q -b "$BRANCH"
git add -A
# Identity for CI/local publish commits (does not rewrite global git config).
git -c user.name='goodtek oss-sync' -c user.email='oss-sync@goodtek.xyz' \
  commit -q -m "$MSG"

git remote add origin "$REMOTE_URL"
echo "==> push $REMOTE_URL ($BRANCH) — $MSG"
# Mirror-style publish: private tree is SoT; public history is sync snapshots.
git push -u --force origin "HEAD:${BRANCH}"

echo "published https://github.com/goodtekxyz/openllms ($SHORT)"
