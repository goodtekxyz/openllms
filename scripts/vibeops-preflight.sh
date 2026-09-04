#!/usr/bin/env bash
# VibeOps preflight — enforce TASK + branch workflow before code changes or commits.
# Usage:
#   scripts/vibeops-preflight.sh           # agent/human check (exit 1 on blockers)
#   scripts/vibeops-preflight.sh --ci      # CI: stricter, no color
#   scripts/vibeops-preflight.sh --commit  # pre-commit: allow docs/tasks-only on integration branch
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CI=false
COMMIT_HOOK=false
for arg in "$@"; do
  case "$arg" in
    --ci) CI=true ;;
    --commit) COMMIT_HOOK=true ;;
  esac
done

red() { if $CI; then echo "ERROR: $*"; else echo -e "\033[31m✗\033[0m $*"; fi; }
green() { if $CI; then echo "OK: $*"; else echo -e "\033[32m✓\033[0m $*"; fi; }
warn() { if $CI; then echo "WARN: $*"; else echo -e "\033[33m!\033[0m $*"; fi; }
info() { echo "  $*"; }

fail=0

integration="$(node -e "console.log(JSON.parse(require('fs').readFileSync('.vibeops.json','utf8')).git.integrationBranch||'develop')" 2>/dev/null || echo develop)"
production="$(node -e "console.log(JSON.parse(require('fs').readFileSync('.vibeops.json','utf8')).git.productionBranch||'main')" 2>/dev/null || echo main)"

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo HEAD)"
if [[ "$branch" == "HEAD" ]]; then
  branch="(detached)"
fi

# --- vibeops CLI (optional in CI if only checking TASK files) ---
if command -v vibeops >/dev/null 2>&1; then
  green "vibeops CLI $(vibeops --version 2>/dev/null | tr -d '\n')"
else
  warn "vibeops CLI not installed — run: npm install -g @goodtek/vibeops"
  info "Docs: docs/ops/VIBEOPS-WORKFLOW.md"
fi

# --- Count In Progress TASKs ---
staged="$(git diff --cached --name-only 2>/dev/null || true)"
ship_metadata_only=false
if $COMMIT_HOOK && [[ -n "$staged" ]]; then
  non_task_staged="$(echo "$staged" | grep -Ev '^docs/tasks/TASK-[0-9]+-.*\.md$' || true)"
  if [[ -z "$non_task_staged" ]]; then
    ship_metadata_only=true
  fi
fi

mapfile -t in_progress < <(grep -rl '^In Progress$' docs/tasks/TASK-*.md 2>/dev/null | grep -Ev 'TASK-000-template\.md$' | sort || true)
count="${#in_progress[@]}"

echo ""
echo "Branch: $branch (integration=$integration, production=$production)"
echo "In Progress TASKs: $count"

if [[ "$count" -eq 0 ]]; then
  if $ship_metadata_only; then
    green "TASK-only commit (ship/reship metadata) — OK"
  else
    red "No In Progress TASK — run: vibeops task add"
    info "Agents must open a TASK before changing product code or ops config."
    fail=1
  fi
elif [[ "$count" -gt 1 ]]; then
  red "Multiple In Progress TASKs ($count) — only one allowed:"
  for f in "${in_progress[@]}"; do info "  $(basename "$f" .md)"; done
  info "Ship or close extras: vibeops task ship TASK-NNN && vibeops task merge TASK-NNN"
  fail=1
else
  task_file="${in_progress[0]}"
  task_id="$(basename "$task_file" | sed -n 's/TASK-\([0-9][0-9][0-9]\).*/\1/p')"
  green "Active TASK: TASK-${task_id} ($(basename "$task_file"))"
fi

# --- Integration / production branch guards ---
if [[ "$branch" == "$integration" || "$branch" == "$production" ]]; then
  if $COMMIT_HOOK; then
    # Allow TASK markdown + governance-only paths on integration when shipping housekeeping
    non_governance="$(echo "$staged" | grep -Ev '^(docs/tasks/|docs/project/05-current-state\.md|docs/ops/VIBEOPS-WORKFLOW\.md|\.cursor/rules/|scripts/vibeops-preflight\.sh|scripts/hooks/|scripts/install-git-hooks\.sh|AGENTS\.md|Makefile)$' || true)"
    if [[ -n "$non_governance" ]]; then
      red "Direct commits to $branch are blocked — use task branch + vibeops task ship/merge"
      echo "$non_governance" | while read -r line; do [[ -n "$line" ]] && info "  $line"; done
      fail=1
    fi
  else
    if ! git diff --quiet || ! git diff --cached --quiet; then
      red "Dirty working tree on $branch — checkout task/* or cursor/* branch first"
      info "  git checkout -b task/NNN-slug   # or resume existing task branch"
      fail=1
    else
      warn "On $branch with clean tree — start work with: vibeops task add (or checkout task branch)"
    fi
  fi
fi

if [[ "$branch" == "$production" ]] && ! $COMMIT_HOOK; then
  red "Do not develop on $production — merge via: vibeops task release (→ deploy-prod CI)"
  fail=1
fi

# --- Deploy path reminder ---
if [[ "$branch" == "$integration" ]] && ! $COMMIT_HOOK && [[ "$fail" -eq 0 ]]; then
  info "Dev deploy: merge to $integration → deploy-dev.yml (automatic)"
  info "Prod deploy: vibeops task release → $production → deploy-prod.yml (never push prod by hand)"
fi

echo ""
if [[ "$fail" -ne 0 ]]; then
  if command -v vibeops >/dev/null 2>&1; then
    info "Hint: vibeops status"
  fi
  exit 1
fi

green "Preflight OK"
exit 0
