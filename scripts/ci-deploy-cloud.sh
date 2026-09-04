#!/usr/bin/env bash
# Deploy Cloud gateway on the self-hosted runner host (Podman Compose).
# Usage: scripts/ci-deploy-cloud.sh <dev|prod> [git-sha]
set -euo pipefail

TARGET="${1:-}"
SHA="${2:-${GITHUB_SHA:-}}"
REPO_DIR="${LLMS_REPO_DIR:-/home/web/goodtek/llms}"

if [[ "$TARGET" != "dev" && "$TARGET" != "prod" ]]; then
  echo "usage: $0 <dev|prod> [git-sha]" >&2
  exit 2
fi

if [[ -z "$SHA" ]]; then
  echo "missing deploy SHA (pass arg or set GITHUB_SHA)" >&2
  exit 2
fi

if [[ ! -d "$REPO_DIR/.git" ]]; then
  echo "LLMS_REPO_DIR is not a git checkout: $REPO_DIR" >&2
  exit 1
fi

cd "$REPO_DIR"

echo "==> syncing $REPO_DIR to $SHA"
git fetch --prune origin
git checkout --force --detach "$SHA"

if [[ "$TARGET" == "dev" ]]; then
  COMPOSE_FILE="deploy/podman-compose.dev.yaml"
  PROJECT="llms-dev"
  ENV_FILE=".env.dev"
  HEALTH_URL="${LLMS_DEV_HEALTH_URL:-https://dev-llms.goodtek.xyz/health}"
  LOCAL_HEALTH="http://127.0.0.1:8082/health"
  DATA_HINT="data-dev"
else
  COMPOSE_FILE="deploy/podman-compose.yaml"
  PROJECT="llms"
  ENV_FILE=".env"
  HEALTH_URL="${LLMS_PROD_HEALTH_URL:-https://llms.goodtek.xyz/health}"
  LOCAL_HEALTH="http://127.0.0.1:8080/health"
  DATA_HINT="data"
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing env file: $REPO_DIR/$ENV_FILE" >&2
  exit 1
fi

mkdir -p "$DATA_HINT/postgres" "$DATA_HINT/dist"

echo "==> building CLI dist into $DATA_HINT/dist"
bash scripts/build-cli-dist.sh "$DATA_HINT/dist" "$SHA"

echo "==> podman compose up --build ($TARGET)"
# Do NOT `source` the env file — values often contain shell-metacharacters.
# Compose interpolates from --env-file alone.
podman compose -f "$COMPOSE_FILE" -p "$PROJECT" --env-file "$ENV_FILE" up --build -d

echo "==> waiting for local health: $LOCAL_HEALTH"
ok=0
for i in $(seq 1 60); do
  if curl -fsS "$LOCAL_HEALTH" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 2
done
if [[ "$ok" -ne 1 ]]; then
  echo "local health check failed after deploy" >&2
  podman compose -f "$COMPOSE_FILE" -p "$PROJECT" ps || true
  exit 1
fi

# Dist is optional for health but required for install.sh happy path.
LOCAL_DIST="${LOCAL_HEALTH%/health}/dist/llms_linux_arm64"
if [[ "$(uname -m)" == "x86_64" ]]; then
  LOCAL_DIST="${LOCAL_HEALTH%/health}/dist/llms_linux_amd64"
fi
echo "==> checking CLI dist: $LOCAL_DIST"
if ! curl -fsSIL "$LOCAL_DIST" >/dev/null 2>&1; then
  echo "warning: CLI dist not reachable at $LOCAL_DIST (install.sh may fall back to GitHub)" >&2
fi

echo "==> public health: $HEALTH_URL"
if ! curl -fsS "$HEALTH_URL" >/dev/null; then
  echo "public health check failed (Caddy/DNS?); local gateway is up" >&2
  exit 1
fi

echo "deploy $TARGET OK @ $SHA"
