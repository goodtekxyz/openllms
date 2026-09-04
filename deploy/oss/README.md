# OSS Quickstart — local only

One container on **127.0.0.1:8080**. SQLite + file secrets. No `.env` copy needed.

For always-on / no-ops hosting: [llms.goodtek.xyz](https://llms.goodtek.xyz).

## Run

```bash
docker compose -f deploy/oss/docker-compose.yml up --build
```

Health:

```bash
curl -sS http://127.0.0.1:8080/health
curl -sS http://127.0.0.1:8080/ready
```

## First success (API-key vendor)

Bootstrap token is the compose default `local-dev-bootstrap` (local only).

1. Bootstrap a project + gateway API key:

```bash
curl -sS -X POST http://127.0.0.1:8080/control/v1/bootstrap \
  -H "X-Bootstrap-Token: local-dev-bootstrap" \
  -H 'Content-Type: application/json' \
  -d '{"login":"me","project_name":"default","key_name":"cli"}'
# save api_key from the response
```

2. Add an upstream account (OpenAI-compatible API key):

```bash
curl -sS -X POST http://127.0.0.1:8080/control/v1/accounts \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"vendor":"openai","name":"main","api_key":"sk-..."}'
# save id as ACCOUNT_ID
```

3. Create a route and attach the account:

```bash
curl -sS -X POST http://127.0.0.1:8080/control/v1/routes \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"slug":"default","strategy":"sequential","default_model":"gpt-4o-mini"}'

curl -sS -X POST http://127.0.0.1:8080/control/v1/routes/default/accounts \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d "{\"account_id\":\"$ACCOUNT_ID\",\"position\":0,\"weight\":1}"
```

4. Call the OpenAI-compatible surface:

```bash
curl -sS http://127.0.0.1:8080/r/default/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'
```

## Automated smoke

```bash
./deploy/oss/smoke.sh
go test ./internal/httpserver/ -run TestOSSE2E -count=1 -v
```

## Data layout

| Path | Purpose |
|------|---------|
| `/data/llms.db` | SQLite database |
| `/data/secrets/` | File vault (`0600` files) |

Back up both for a full restore.
