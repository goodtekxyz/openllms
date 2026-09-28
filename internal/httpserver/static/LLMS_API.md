# llms Unified API

> Canonical client entrance for agents and OpenAI-compatible clients.
> Gateway: `https://llms.goodtek.xyz`

## Standard endpoint (use this only)

```http
POST https://llms.goodtek.xyz/r/{route_slug}/v1/chat/completions
Authorization: Bearer sk-gt-…
Content-Type: application/json
```

Clients send **OpenAI Chat Completions** JSON. The gateway selects an account from the route pool, translates request/response per provider, and returns OpenAI-shaped output plus optional `llms` extensions.

Do **not** call provider-specific paths (`/responses`, raw Codex URLs) from product clients.

## Request

### Minimal chat

```json
{
  "model": "gpt-5.4-mini",
  "messages": [
    { "role": "system", "content": "You are helpful." },
    { "role": "user", "content": "Hello" }
  ],
  "stream": true
}
```

`model` must be a route model id from `GET /r/{slug}/v1/models`.

### Function tools (client catalog)

Send only OpenAI function tools:

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "web_search",
        "description": "Search the web for current facts",
        "parameters": {
          "type": "object",
          "properties": { "query": { "type": "string" } },
          "required": ["query"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

| Client function name | Gateway behavior (Codex OAuth) | Gateway behavior (Claude OAuth/API key) | Gateway behavior (OpenAI API key) |
|--------------------|----------------------------------|----------------------------------------|-----------------------------------|
| `web_search` | Maps to hosted `web_search` upstream | Maps to hosted `web_search_20250305` upstream | Maps to hosted `web_search` via `/responses` |
| `generate_image` | Maps to hosted `image_generation` upstream | Passed as custom tool (no hosted image gen) | Maps to hosted `image_generation` via `/responses` |
| `browser_search`, others | Passed as `{type:function}` upstream | Passed as custom tool (`input_schema`) | Passed as `{type:function}` upstream |

Clients **must not** send Codex `{type:web_search}` or Claude tool types.

### Optional `llms` hints

```json
{
  "llms": {
    "web_search": { "mode": "live" },
    "session_key": "thread-abc"
  }
}
```

| Field | Values | Default |
|-------|--------|---------|
| `llms.web_search.mode` | `live`, `cached`, `disabled` | `live` |
| `llms.session_key` | string | sticky routing hint |

## Response

### Non-stream JSON

OpenAI chat completion shape, plus:

```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "…",
      "tool_calls": []
    },
    "finish_reason": "stop"
  }],
  "llms": {
    "account_id": "…",
    "provider": "codex",
    "hosted_tools": [
      {
        "name": "web_search",
        "status": "completed",
        "action": { "type": "search", "query": "…" }
      }
    ]
  }
}
```

- **Client-executed tools** → `choices[].message.tool_calls`
- **Gateway-executed hosted tools** (Codex web search, image gen) → `llms.hosted_tools`

### Image result (hosted)

```json
"llms": {
  "hosted_tools": [{
    "name": "generate_image",
    "status": "completed",
    "result": {
      "b64_json": "<base64>",
      "mime_type": "image/png"
    }
  }]
}
```

### Streaming SSE

OpenAI `chat.completion.chunk` events, plus optional:

```json
{"llms":{"hosted_tool":{"phase":"started","name":"web_search","id":"ws_…"}}}
{"llms":{"hosted_tool":{"phase":"completed","name":"web_search","status":"completed","action":{…}}}}
```

## Compatibility endpoints (not for new clients)

| Endpoint | Notes |
|----------|-------|
| `POST /r/{slug}/v1/messages` | Anthropic-native clients only |
| `POST /r/{slug}/v1/images/generations` | Legacy facade for API-key OpenAI; prefer chat + `generate_image` tool (also uses `/responses` upstream) |
| `POST /r/{slug}/v1/responses` | Not exposed — internal upstream only |

## Discovery

- `GET /control/v1/meta` → `unified_api_doc`, `api_surface`
- `GET /r/{slug}/v1/models` → routable models
- Install: `/install.md`, `/install.sh`

## Example (web search)

```bash
curl -sS "https://llms.goodtek.xyz/r/MY_ROUTE/v1/chat/completions" \
  -H "Authorization: Bearer sk-gt-…" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-5.4-mini",
    "messages":[{"role":"user","content":"Who is the current CEO of DB Asset Management?"}],
    "tools":[{"type":"function","function":{"name":"web_search","description":"search","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}}],
    "tool_choice":"auto",
    "stream":false
  }'
```

Expect grounded `content` and `llms.hosted_tools` on Codex OAuth, Claude, or OpenAI API-key routes with `web_search`.
