# openllms

**Use your ChatGPT / Claude subscription as an API.**  
Check quotas. If you have several accounts, **spread the calls** across them.

If you’re already paying for Plus, Pro, or Claude, you can put those logins behind one OpenAI-compatible base URL for Cursor, Claude Code, Codex, and similar tools. The gateway watches remaining quota and routes traffic so you don’t burn one account while others sit idle.

This is the self-hosted open-source engine behind [llms](https://llms.goodtek.xyz), from [goodtek](https://goodtek.xyz).

[한국어](README.ko.md) · [License (MIT)](LICENSE) · [Hosted version](https://llms.goodtek.xyz)

---

## The core idea

1. **Subscription → API**  
   Put ChatGPT / Claude (or Codex) subscription sessions behind an OpenAI-shaped base URL. Your tools only need the gateway URL and a gateway API key.

2. **Quota awareness**  
   See what’s left per account so you don’t keep hammering something that’s already out of allowance.

3. **Fair routing across accounts**  
   Attach several accounts to a route and spread calls instead of dumping everything on one login. When one hits a limit or fails, traffic moves to another.

Quota shows up in the CLI like this (`llms status` / `llms status --refresh`):

```text
ACCOUNTS
  ●  claude:work           ok        quota  78% ████████░░  reset ~2d
  ●  chatgpt:personal      ok        quota  54% █████░░░░░  reset ~5d
  ●  codex:team            cooling   quota  18% ██░░░░░░░░  reset ~14h
  ●  claude:spare          low       quota   6% █░░░░░░░░░  reset ~2d
```

Accounts with less left get deprioritized when routing.

---

## Why bother

You’re already paying for a subscription, but coding tools want an API.  
With more than one account, every day starts with “which login do I use this time?”

Spreadsheets and shell aliases get messy. openllms turns that into **one URL, one key, and routes you control**.

## Quick start

Local only. Docker is enough — no `.env` copy.

```bash
git clone https://github.com/goodtekxyz/openllms.git
cd openllms
docker compose -f deploy/oss/docker-compose.yml up --build
curl -sS http://127.0.0.1:8080/health
```

Bootstrap → account → route → first call: [`deploy/oss/README.md`](deploy/oss/README.md).

Want it always on without running the box yourself? Use [llms.goodtek.xyz](https://llms.goodtek.xyz).

### Build without Docker

```bash
go build -o bin/llms-gateway ./cmd/llms-gateway
go build -o bin/llms ./cmd/llms
```

## What’s in this repo

| Path | What it is |
|------|------------|
| `cmd/llms-gateway` | HTTP gateway |
| `cmd/llms` | Ops CLI |
| `internal/` | Routing, account wiring, and the rest of the core |
| `deploy/oss/` | Docker setup example for running it yourself |

What’s public here is the gateway itself. Hosted-only pieces (billing, admin UI) are not in this repo.

## Contributing

Issues and PRs that improve the shared engine are welcome. Don’t commit secrets or private hostnames. We check the OSS Quickstart first.

## License

[MIT](LICENSE). Use it, fork it, ship it.

---

## About goodtek

We’re [goodtek](https://goodtek.xyz). We build AI and automation people actually run — **llms** (hosted gateway) and **vibePulse** (uptime / heartbeat monitoring). We also run [VibeCrew](https://vibecrew.kr), a vibe-coding builder community.

| | |
|---|---|
| Website | [goodtek.xyz](https://goodtek.xyz) |
| Email | [hello@goodtek.xyz](mailto:hello@goodtek.xyz) · [contact page](https://goodtek.xyz/contact) |
| Hosted llms | [llms.goodtek.xyz](https://llms.goodtek.xyz) |
| X | [@goodtekxyz](https://x.com/goodtekxyz) |
| Threads | [@goodtek.xyz](https://www.threads.com/@goodtek.xyz) |
| VibeCrew | [vibecrew.kr](https://vibecrew.kr) |

Questions, ideas, partnerships — **hello@goodtek.xyz**. A real person reads it.
