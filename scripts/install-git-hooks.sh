#!/usr/bin/env bash
# Install repo git hooks (VibeOps preflight on commit).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
mkdir -p .git/hooks
install -m 755 "$ROOT/scripts/hooks/pre-commit" "$ROOT/.git/hooks/pre-commit"
echo "Installed .git/hooks/pre-commit → scripts/vibeops-preflight.sh --commit"
