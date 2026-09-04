# syntax=docker/dockerfile:1
FROM docker.io/library/golang:1.24.5-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Cloud images default to -tags cloud (Infisical/admin/billing overlay).
# OSS Quickstart passes BUILD_TAGS= (empty) via compose build args.
ARG BUILD_TAGS=cloud
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
	-tags "${BUILD_TAGS}" \
	-o /out/llms-gateway ./cmd/llms-gateway

FROM docker.io/library/alpine:3.21
RUN adduser -D -u 65532 nonroot
COPY --from=build /out/llms-gateway /llms-gateway
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/llms-gateway"]
