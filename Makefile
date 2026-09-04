.PHONY: test tidy build run compose-up compose-down health

export PATH := /usr/local/go/bin:$(PATH)

test:
	go test ./...

tidy:
	go mod tidy

build:
	go build -o bin/llms-gateway ./cmd/llms-gateway
	go build -ldflags "-X main.Version=$$(git describe --tags --always 2>/dev/null || echo dev) -X main.Commit=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" -o bin/llms ./cmd/llms

.PHONY: build-cli-dist
build-cli-dist:
	bash scripts/build-cli-dist.sh $${OUT:-./data/dist}

run: build
	DATABASE_URL=$${DATABASE_URL:-postgres://llms:llms@127.0.0.1:54329/llms?sslmode=disable} \
	HTTP_ADDR=$${HTTP_ADDR:-:8080} \
	./bin/llms-gateway

compose-up:
	podman compose -f deploy/podman-compose.yaml up --build -d

compose-down:
	podman compose -f deploy/podman-compose.yaml down

health:
	curl -sf http://127.0.0.1:8080/health
	@echo
	curl -sf http://127.0.0.1:8080/ready
	@echo

.PHONY: build-oss build-cloud oss-sync oss-sync-check oss-publish

# Default binary: public-first OSS (file secrets; no cloud/)
build-oss:
	go build -o bin/llms-gateway ./cmd/llms-gateway
	go build -o bin/llms ./cmd/llms

# Cloud binary: Cloud overlay (Infisical + admin/billing)
build-cloud:
	go build -tags cloud -o bin/llms-gateway-cloud ./cmd/llms-gateway

oss-sync:
	./scripts/oss-sync.sh

oss-sync-check: oss-sync
	./scripts/oss-sync-check.sh .oss-export

# Sync + check + push to github.com/goodtekxyz/openllms (requires git push access).
oss-publish:
	bash scripts/oss-publish.sh

oss-publish-dry:
	bash scripts/oss-publish.sh --dry-run

.PHONY: vibeops-check vibeops-hooks

vibeops-check:
	bash scripts/vibeops-preflight.sh

vibeops-hooks:
	bash scripts/install-git-hooks.sh
