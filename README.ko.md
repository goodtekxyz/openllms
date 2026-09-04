# openllms

**ChatGPT·Claude 구독으로 API처럼 쓰기.**  
쿼터 보고, 계정 여러 개면 **골고루** 나눠 호출하기.

이미 내고 있는 Plus / Pro / Claude 구독이 있으면, 그걸 Cursor·Claude Code·Codex 같은 도구의 API 호출에 붙일 수 있습니다. 계정마다 남은 한도를 보고, 라우트가 요청을 계정들에 나눠 줍니다.

셀프호스트용 오픈소스이고, [llms](https://llms.goodtek.xyz)의 엔진입니다. [goodtek](https://goodtek.xyz)이 만듭니다.

[English](README.md) · [라이선스 (MIT)](LICENSE) · [호스팅 버전](https://llms.goodtek.xyz)

---

## 핵심이 이거예요

1. **구독 → API**  
   ChatGPT / Claude 쪽 구독(또는 Codex 로그인)을 OpenAI 호환 Base URL 뒤에 둡니다. 도구에는 게이트웨이 주소와 키만 넣으면 됩니다.

2. **쿼터 확인**  
   계정별 남은 한도를 보고, 바닥난 쪽으로 요청을 계속 보내지 않게 돕습니다.

3. **여러 계정이면 골고루**  
   라우트에 계정을 여러 개 붙이면, 호출을 한쪽에만 몰지 않고 나눠 씁니다. 한도가 닿거나 막히면 다른 계정으로 넘깁니다.

한도는 CLI에서 이렇게 보입니다 (`llms status` / `llms status --refresh`):

```text
ACCOUNTS
  ●  claude:work           ok        quota  78% ████████░░  reset ~2d
  ●  chatgpt:personal      ok        quota  54% █████░░░░░  reset ~5d
  ●  codex:team            cooling   quota  18% ██░░░░░░░░  reset ~14h
  ●  claude:spare          low       quota   6% █░░░░░░░░░  reset ~2d
```

남은 %가 낮은 계정은 라우팅에서 뒤로 밀립니다.

---

## 왜 필요하냐면

구독은 이미 있는데, 코딩 도구는 API 키를 달라고 하고…  
계정을 여러 개 쓰면 “이번엔 어느 계정으로 보내지?”가 매일 생깁니다.

엑셀·셸 별칭으로 버티다 보면 금방 꼬입니다. openllms는 **URL 하나 + 키 하나 + 라우트**로 그 부분을 맡깁니다.

## 빠르게 해 보기

로컬 전용입니다. Docker만 있으면 됩니다 (`.env` 복사 없음).

```bash
git clone https://github.com/goodtekxyz/openllms.git
cd openllms
docker compose -f deploy/oss/docker-compose.yml up --build
curl -sS http://127.0.0.1:8080/health
```

부트스트랩 → 계정 → 라우트 → 첫 호출은 [`deploy/oss/README.md`](deploy/oss/README.md)에 있습니다.

서버를 항상 켜 두고 운영까지 맡기고 싶으면 [llms.goodtek.xyz](https://llms.goodtek.xyz) 호스팅을 쓰면 됩니다.

### Docker 없이 빌드

```bash
go build -o bin/llms-gateway ./cmd/llms-gateway
go build -o bin/llms ./cmd/llms
```

## 이 저장소에 뭐가 있나

| 경로 | 내용 |
|------|------|
| `cmd/llms-gateway` | HTTP 게이트웨이 |
| `cmd/llms` | 운영 CLI |
| `internal/` | 라우팅, 계정 연동 등 핵심 코드 |
| `deploy/oss/` | 직접 돌릴 때 쓰는 Docker 설정 예제 |

여기 공개된 건 게이트웨이 본체입니다. 결제·관리자 화면 같은 호스팅 전용 기능은 이 저장소에 없습니다.

## 기여

엔진을 나아지게 하는 이슈·PR은 환영합니다. 시크릿·사내 호스트 이름은 커밋하지 말아 주세요. OSS Quickstart가 깨지는지 먼저 봅니다.

## 라이선스

[MIT](LICENSE). 쓰고, 포크하고, 제품에 넣어도 됩니다.

---

## goodtek 소개

[goodtek](https://goodtek.xyz)입니다. 매일 실제로 돌아가는 AI·자동화 제품을 만듭니다. **llms**(이 게이트웨이의 호스팅), **vibePulse**(업타임·heartbeat 모니터)가 있고, 바이브코딩 커뮤니티 [VibeCrew](https://vibecrew.kr)도 운영합니다.

| | |
|---|---|
| 웹사이트 | [goodtek.xyz](https://goodtek.xyz) |
| 메일 | [hello@goodtek.xyz](mailto:hello@goodtek.xyz) · [문의 페이지](https://goodtek.xyz/contact) |
| 호스팅 llms | [llms.goodtek.xyz](https://llms.goodtek.xyz) |
| X | [@goodtekxyz](https://x.com/goodtekxyz) |
| Threads | [@goodtek.xyz](https://www.threads.com/@goodtek.xyz) |
| VibeCrew | [vibecrew.kr](https://vibecrew.kr) |

궁금한 점, 아이디어, 제휴는 **hello@goodtek.xyz** 로 보내 주세요. 사람이 직접 읽습니다.
