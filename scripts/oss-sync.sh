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
  --exclude='.personas/' \
  --exclude='.vibeops/' \
  --exclude='.vibeops.json' \
  --exclude='.vibeops.env' \
  --exclude='.vibeops.env.example' \
  --exclude='CLAUDE.md' \
  --exclude='node_modules/' \
  --exclude='artifacts/' \
  --exclude='data/' \
  --exclude='data-*/' \
  --exclude='bin/' \
  --exclude='.env' \
  --exclude='.env.*' \
  --exclude='AGENTS.md' \
  --exclude='.github/' \
  --exclude='scripts/vibeops-preflight.sh' \
  --exclude='scripts/ci-deploy-cloud.sh' \
  --exclude='scripts/billing-rails-evidence.sh' \
  --exclude='scripts/backup-postgres.sh' \
  --exclude='scripts/capture-billing-ui.mjs' \
  --exclude='scripts/capture-console-ui.mjs' \
  --exclude='scripts/capture-site-ui.mjs' \
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
  ".personas"
  ".vibeops"
  ".vibeops.json"
  ".vibeops.env"
  ".vibeops.env.example"
  "CLAUDE.md"
  "scripts/vibeops-preflight.sh"
  "scripts/ci-deploy-cloud.sh"
  "scripts/billing-rails-evidence.sh"
  "scripts/backup-postgres.sh"
  "scripts/capture-billing-ui.mjs"
  "scripts/capture-console-ui.mjs"
  "scripts/capture-site-ui.mjs"
)

for p in "${DENY[@]}"; do
  rm -rf "$DEST/$p"
done

# Portable in-place sed (GNU sed -i vs BSD sed -i '').
sed_i() {
  if sed --version >/dev/null 2>&1; then
    sed -i "$@"
  else
    local expr=$1
    shift
    sed -i '' "$expr" "$@"
  fi
}

# Scrub module path + Infisical host defaults.
if [[ -f "$DEST/go.mod" ]]; then
  sed_i "s|${MODULE_FROM}|${MODULE_TO}|g" "$DEST/go.mod"
fi
while IFS= read -r -d '' f; do
  sed_i "s|${MODULE_FROM}|${MODULE_TO}|g" "$f"
done < <(find "$DEST" -type f \( -name '*.go' -o -name '*.md' -o -name 'go.mod' \) -print0)
while IFS= read -r -d '' f; do
  if sed --version >/dev/null 2>&1; then
    sed -i \
      -e 's|https://infisical\.goodtek\.xyz|http://127.0.0.1:8080|g' \
      -e 's|infisical\.goodtek\.xyz|localhost|g' \
      "$f"
  else
    sed -i '' \
      -e 's|https://infisical\.goodtek\.xyz|http://127.0.0.1:8080|g' \
      -e 's|infisical\.goodtek\.xyz|localhost|g' \
      "$f"
  fi
done < <(find "$DEST" -type f \( -name '*.go' -o -name '*.md' -o -name '*.example' \) -print0)

cp "$ROOT/docs/oss/LICENSE" "$DEST/LICENSE"
cp "$ROOT/docs/oss/README.md" "$DEST/README.md"
cp "$ROOT/docs/oss/README.ko.md" "$DEST/README.ko.md"
cp "$ROOT/docs/oss/README.ja.md" "$DEST/README.ja.md"
cp "$ROOT/docs/oss/README.zh.md" "$DEST/README.zh.md"

echo "oss-sync wrote $DEST (public-first; cloud/ excluded)"
echo "next: ./scripts/oss-sync-check.sh $DEST"
