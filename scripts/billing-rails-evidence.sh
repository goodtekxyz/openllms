#!/usr/bin/env bash
# Evidence harness for Polar/Unifi billing rails (read-only checks).
# Usage:
#   BASE_URL=https://dev-llms.goodtek.xyz ./scripts/billing-rails-evidence.sh
#   BASE_URL=https://llms.goodtek.xyz ./scripts/billing-rails-evidence.sh
set -euo pipefail

BASE_URL="${BASE_URL:-https://dev-llms.goodtek.xyz}"
BASE_URL="${BASE_URL%/}"

echo "== billing rails evidence =="
echo "base: $BASE_URL"
echo

health=$(curl -fsS -o /tmp/llms-health.json -w '%{http_code}' "$BASE_URL/health" || true)
echo "GET /health -> $health $(tr '\n' ' ' </tmp/llms-health.json 2>/dev/null || true)"
if [[ "$health" != "200" ]]; then
  echo "FAIL: health"
  exit 1
fi

meta=$(curl -fsS "$BASE_URL/control/v1/meta")
echo "$meta" | tee /tmp/llms-meta.json | jq '{
  billing_mock,
  billing_enforce,
  billing,
  billing_rails
}' 2>/dev/null || echo "$meta"

billing_page=$(curl -fsS -o /tmp/llms-billing.html -w '%{http_code}' "$BASE_URL/billing" || true)
echo "GET /billing -> $billing_page"
if [[ "$billing_page" != "200" ]]; then
  echo "FAIL: billing page"
  exit 1
fi

# Unauthorized checkout must not 5xx
chk=$(curl -sS -o /tmp/llms-checkout.json -w '%{http_code}' \
  -X POST "$BASE_URL/billing/api/checkout" \
  -H 'content-type: application/json' \
  -d '{"plan":"starter","provider":"polar"}' || true)
echo "POST /billing/api/checkout (no session) -> $chk $(tr '\n' ' ' </tmp/llms-checkout.json)"
case "$chk" in
  401|403|400) ;;
  *) echo "WARN: unexpected checkout status (want 401/403 without session)" ;;
esac

# Webhook without valid signature: when secret configured expect 401; when empty may 200/400
polar_wh=$(curl -sS -o /tmp/llms-polar-wh.json -w '%{http_code}' \
  -X POST "$BASE_URL/billing/webhooks/polar" \
  -H 'content-type: application/json' \
  -H 'Polar-Signature: v1=deadbeef' \
  -d '{"type":"subscription.active","data":{}}' || true)
echo "POST /billing/webhooks/polar (bad sig) -> $polar_wh"

unifi_wh=$(curl -sS -o /tmp/llms-unifi-wh.json -w '%{http_code}' \
  -X POST "$BASE_URL/billing/webhooks/unifi" \
  -H 'content-type: application/json' \
  -H 'X-Timestamp: 1' \
  -H 'X-Authorization-Hmac: nope' \
  -d '{"status":"paid","requestId":"x"}' || true)
echo "POST /billing/webhooks/unifi (bad sig) -> $unifi_wh"

echo
echo "== interpretation =="
jq -r '
  "billing_mock=" + (.billing_mock|tostring) +
  " billing_enforce=" + (.billing_enforce|tostring) +
  " polar_live=" + ((.billing_rails.polar_live // false)|tostring) +
  " unifi_live=" + ((.billing_rails.unifi_live // false)|tostring)
' /tmp/llms-meta.json 2>/dev/null || true

echo
echo "Next human steps when polar_live/unifi_live are false:"
echo "  fill POLAR_* / UNIFI_PAY_* per docs/ops/HUMAN-SETUP.md §6"
echo "  set BILLING_MOCK=false on that environment"
echo "  re-run this script; then do a real checkout smoke"
