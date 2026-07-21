# syntax=docker/dockerfile:1.7
# Multi-arch build: pass --platform=linux/amd64,linux/arm64 to buildx.
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.20
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# --- Build stage -----------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X github.com/sachncs/promptsheon/internal/buildinfo.Version=${VERSION} -X github.com/sachncs/promptsheon/internal/buildinfo.Commit=${COMMIT} -X github.com/sachncs/promptsheon/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/promptsheond ./cmd/promptsheond && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X github.com/sachncs/promptsheon/internal/buildinfo.Version=${VERSION} -X github.com/sachncs/promptsheon/internal/buildinfo.Commit=${COMMIT} -X github.com/sachncs/promptsheon/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/promptsheon ./cmd/promptsheon && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w" \
      -o /out/promptsheon-healthcheck ./cmd/promptsheon-healthcheck

# --- Runtime stage ---------------------------------------------------------
FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -g 1000 promptsheon && adduser -D -u 1000 -G promptsheon promptsheon

# /data holds the SQLite database and is the volume target. The
# binary runs as UID 1000 so the directory must be owned by the
# same user. The chart mounts a PVC at the same path.
WORKDIR /data
RUN chown -R promptsheon:promptsheon /data
COPY --from=builder /out/promptsheond           /usr/local/bin/promptsheond
COPY --from=builder /out/promptsheon            /usr/local/bin/promptsheon
COPY --from=builder /out/promptsheon-healthcheck /usr/local/bin/promptsheon-healthcheck

# SEC-CONTAINER-1: OCI image labels. Builders and registries use
# these for provenance, mirror routing, and CVE traceability.
LABEL org.opencontainers.image.source="https://github.com/sachncs/promptsheon" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.title="promptsheon"

USER promptsheon

ENV PROMPTSHEON_ADDR=:8080 \
    PROMPTSHEON_DB_PATH=/data/promptsheon.db \
    HEALTHCHECK_PORT=8080

EXPOSE 8080
VOLUME ["/data"]

# SEC-CONTAINER-2: Go-based healthcheck replaces wget. The
# healthcheck binary honours PROMPTSHEON_HEALTHCHECK_HOST /
# _PORT and exits 0 on 200, non-zero otherwise. No shell, no
# wget, no curl, no extra apk packages.
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD PROMPTSHEON_HEALTHCHECK_HOST=localhost \
      PROMPTSHEON_HEALTHCHECK_PORT=${HEALTHCHECK_PORT} \
      /usr/local/bin/promptsheon-healthcheck || exit 1

ENTRYPOINT ["promptsheond"]
