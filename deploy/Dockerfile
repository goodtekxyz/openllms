# syntax=docker/dockerfile:1
FROM docker.io/library/golang:1.24.5-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# GIT_SHA must change every deploy so Podman/Buildah cannot reuse a stale
# COPY/. layer when only embedded static files changed (seen on self-hosted runners).
ARG GIT_SHA=unknown
RUN printf '%s\n' "$GIT_SHA" > /tmp/llms-git-sha
COPY . .
# Cloud images default to -tags cloud (Infisical/admin/billing overlay).
# OSS Quickstart passes BUILD_TAGS= (empty) via compose build args.
ARG BUILD_TAGS=cloud
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
	-tags "${BUILD_TAGS}" \
	-o /out/llms-gateway ./cmd/llms-gateway

FROM docker.io/library/alpine:3.21
RUN apk add --no-cache su-exec \
	&& adduser -D -u 65532 nonroot \
	&& mkdir -p /data/secrets \
	&& chown -R nonroot:nonroot /data
COPY --from=build /out/llms-gateway /llms-gateway
COPY deploy/oss/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod 0755 /docker-entrypoint.sh
# Start as root so the entrypoint can chown named volumes, then su-exec to
# nonroot. Platforms that force runAsNonRoot/uid 65532 skip chown and exec.
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
