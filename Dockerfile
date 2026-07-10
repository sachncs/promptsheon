# Build stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o promptsheond ./cmd/promptsheond
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o promptsheon ./cmd/promptsheon

# Runtime stage
FROM alpine:3.20
# L-8 fix: install wget explicitly for HEALTHCHECK. The base
# alpine image does not bundle wget (the busybox version is
# unreliable across versions), and the previous HEALTHCHECK
# silently failed on minimal images.
RUN apk add --no-cache ca-certificates tzdata wget
RUN adduser -D -u 1000 promptsheon
COPY --from=builder /app/promptsheond /usr/local/bin/promptsheond
COPY --from=builder /app/promptsheon  /usr/local/bin/promptsheon
USER promptsheon
# Default to 8080 to match the documented PROMPTSHEON_ADDR
# default. Operators who set PROMPTSHEON_ADDR to a different
# port must also pass HEALTHCHECK_PORT at runtime (e.g.
# '--env HEALTHCHECK_PORT=9090') so the container healthcheck
# still hits the right endpoint.
ENV PROMPTSHEON_ADDR=:8080
ENV HEALTHCHECK_PORT=8080
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:${HEALTHCHECK_PORT}/health || exit 1
ENTRYPOINT ["promptsheond"]
