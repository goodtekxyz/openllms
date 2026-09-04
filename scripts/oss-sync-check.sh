#!/usr/bin/env bash
# Fail if an OSS export tree contains private overlay / Cloud paths.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${1:-}"

if [[ -z "$DEST" ]]; then
  "$ROOT/scripts/oss-sync.sh"
  DEST="$ROOT/.oss-export"
fi

if [[ ! -d "$DEST" ]]; then
  echo "missing export dir: $DEST" >&2
  exit 1
fi

FAIL=0
assert_absent() {
  local p="$1"
  if [[ -e "$DEST/$p" ]]; then
    echo "DENY leak: $p" >&2
    FAIL=1
  fi
}
assert_present() {
  local p="$1"
  if [[ ! -e "$DEST/$p" ]]; then
    echo "ALLOW missing: $p" >&2
    FAIL=1
  fi
}

assert_absent "cloud"
assert_absent "docs/project"
assert_absent "docs/tasks"
assert_absent "docs/ops"
assert_absent "cmd/llms-gateway/secrets_cloud.go"
assert_absent "internal/httpserver/new_cloud.go"
assert_absent "internal/httpserver/billing_cloud.go"
assert_absent "internal/httpserver/admin.go"
assert_absent "deploy/podman-compose.yaml"

assert_present "cmd/llms-gateway/main.go"
assert_present "cmd/llms-gateway/secrets_default.go"
assert_present "internal/httpserver/new_default.go"
assert_present "internal/httpserver/mount_default.go"
assert_present "internal/secrets/file"
assert_present "internal/db/migrations_sqlite"
assert_present "deploy/oss/README.md"
assert_present "go.mod"
assert_present "LICENSE"
assert_present "README.md"
assert_present "README.ko.md"

if [[ -d "$DEST/cloud" ]]; then
  echo "cloud/ overlay must not ship" >&2
  FAIL=1
fi

if ! grep -q 'github.com/goodtekxyz/openllms' "$DEST/go.mod"; then
  echo "go.mod was not rewritten to openllms" >&2
  FAIL=1
fi

if grep -R -n 'infisical.goodtek.xyz' "$DEST" --include='*.go' --include='*.md' 2>/dev/null | head; then
  echo "scrub missed Infisical host" >&2
  FAIL=1
fi

# Public tree must not import the private overlay package path.
if grep -R -n 'github.com/goodtekxyz/openllms/cloud/' "$DEST" --include='*.go' 2>/dev/null | head; then
  echo "public tree imports cloud/ overlay" >&2
  FAIL=1
fi
if grep -R -n 'github.com/goodtekxyz/llms/cloud/' "$DEST" --include='*.go' 2>/dev/null | head; then
  echo "public tree still references private cloud/ imports" >&2
  FAIL=1
fi

if [[ "$FAIL" -ne 0 ]]; then
  echo "oss-sync-check FAILED" >&2
  exit 1
fi

echo "oss-sync-check OK ($DEST)"
