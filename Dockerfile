# syntax=docker/dockerfile:1.7
# Multi-arch build: pass --platform=linux/amd64,linux/arm64 to buildx.
ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.20

# --- Build stage -----------------------------------------------------------
FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X github.com/sachncs/promptsheon/internal/buildinfo.Version=${VERSION} -X github.com/sachncs/promptsheon/internal/buildinfo.Commit=${COMMIT} -X github.com/sachncs/promptsheon/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/promptsheond ./cmd/promptsheond && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X github.com/sachncs/promptsheon/internal/buildinfo.Version=${VERSION} -X github.com/sachncs/promptsheon/internal/buildinfo.Commit=${COMMIT} -X github.com/sachncs/promptsheon/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/promptsheon ./cmd/promptsheon

# --- Runtime stage ---------------------------------------------------------
FROM alpine:${ALPINE_VERSION}
RUN apk add --no-cache ca-certificates tzdata wget
RUN addgroup -g 1000 promptsheon && adduser -D -u 1000 -G promptsheon promptsheon

# /data holds the SQLite database and is the volume target. The
# binary runs as UID 1000 so the directory must be owned by the
# same user. The chart mounts a PVC at the same path.
WORKDIR /data
RUN chown -R promptsheon:promptsheon /data
COPY --from=builder /out/promptsheond /usr/local/bin/promptsheond
COPY --from=builder /out/promptsheon  /usr/local/bin/promptsheon

USER promptsheon

ENV PROMPTSHEON_ADDR=:8080 \
    PROMPTSHEON_DB_PATH=/data/promptsheon.db \
    HEALTHCHECK_PORT=8080

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:${HEALTHCHECK_PORT}/health || exit 1

ENTRYPOINT ["promptsheond"]
