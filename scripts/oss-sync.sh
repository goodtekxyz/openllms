#!/usr/bin/env bash
# Export the public-first tree for github.com/goodtekxyz/openllms.
# Rule: everything outside private overlay roots is public. Does not push.
# Default dest: .oss-export/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="${1:-"$ROOT/.oss-export"}"
MODULE_FROM="github.com/goodtekxyz/llms"
MODULE_TO="github.com/goodtekxyz/openllms"

rm -rf "$DEST"
mkdir -p "$DEST"

# Public-first: copy the repo, then drop private overlay + internal ops docs.
rsync -a \
  --exclude='.git/' \
  --exclude='.oss-export/' \
  --exclude='cloud/' \
  --exclude='docs/project/' \
  --exclude='docs/tasks/' \
  --exclude='docs/ops/' \
  --exclude='docs/logs/' \
  --exclude='docs/artifacts/' \
  --exclude='deploy/podman-compose.yaml' \
  --exclude='deploy/podman-compose.dev.yaml' \
  --exclude='deploy/Caddyfile.snippet' \
  --exclude='deploy/Caddyfile.dev.snippet' \
  --exclude='.cursor/' \
  --exclude='.cursor-screenshots/' \
  --exclude='node_modules/' \
  --exclude='artifacts/' \
  --exclude='data/' \
  --exclude='data-*/' \
  --exclude='bin/' \
  --exclude='.env' \
  --exclude='.env.*' \
  --exclude='AGENTS.md' \
  --exclude='.github/' \
  "$ROOT/" "$DEST/"

# Defense-in-depth: never ship these even if exclude slips.
DENY=(
  "cloud"
  "docs/project"
  "docs/tasks"
  "docs/ops"
  "deploy/podman-compose.yaml"
  "deploy/podman-compose.dev.yaml"
  "cmd/llms-gateway/secrets_cloud.go"
  "internal/httpserver/new_cloud.go"
  "internal/httpserver/mount_cloud.go"
  "internal/httpserver/billing_cloud.go"
  "internal/httpserver/billing_rails_cloud.go"
  "internal/httpserver/notify_signup_cloud.go"
  "internal/httpserver/admin.go"
  "internal/httpserver/admin_test.go"
)

for p in "${DENY[@]}"; do
  rm -rf "$DEST/$p"
done

# Scrub module path + Infisical host defaults.
if [[ -f "$DEST/go.mod" ]]; then
  sed -i "s|${MODULE_FROM}|${MODULE_TO}|g" "$DEST/go.mod"
fi
find "$DEST" -type f \( -name '*.go' -o -name '*.md' -o -name 'go.mod' \) -print0 |
  xargs -0 -r sed -i "s|${MODULE_FROM}|${MODULE_TO}|g"
find "$DEST" -type f \( -name '*.go' -o -name '*.md' -o -name '*.example' \) -print0 |
  xargs -0 -r sed -i \
    -e 's|https://infisical\.goodtek\.xyz|http://127.0.0.1:8080|g' \
    -e 's|infisical\.goodtek\.xyz|localhost|g'

cp "$ROOT/docs/oss/LICENSE" "$DEST/LICENSE"
cp "$ROOT/docs/oss/README.md" "$DEST/README.md"
cp "$ROOT/docs/oss/README.ko.md" "$DEST/README.ko.md"

echo "oss-sync wrote $DEST (public-first; cloud/ excluded)"
echo "next: ./scripts/oss-sync-check.sh $DEST"
