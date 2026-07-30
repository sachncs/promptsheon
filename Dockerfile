# Dockerfile
#
# Multi-stage source build for `docker build .`. The release
# path is Dockerfile.goreleaser (a slimmer binary-only
# image consumed by GoReleaser); this file is the canonical
# build for `docker build` / `docker compose` / CI smoke
# images where the source tree is present.
#
# Usage:
#   docker build -t promptsheon:dev .
#   docker run --rm -p 8080:8080 promptsheon:dev
#
# The image builds all three binaries into /usr/local/bin/;
# ENTRYPOINT is promptsheond (the daemon), but operators can
# override with `docker run --entrypoint promptsheon ...` for
# CLI use or `--entrypoint promptsheon-healthcheck ...` for
# the probe.

# syntax=docker/dockerfile:1.7

# ----- Build stage ---------------------------------------------------------
FROM golang:1.26.5-alpine3.20 AS build
WORKDIR /src

# Cache go.mod first to maximise layer reuse.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source.
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG COMMIT_DATE=unknown

# Build all three binaries. main.go dispatches by os.Args[0],
# so each binary needs the right name to land in the right
# mode at runtime.
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/sachncs/promptsheon/backend.Version=${VERSION} -X github.com/sachncs/promptsheon/backend.Commit=${COMMIT} -X github.com/sachncs/promptsheon/backend.BuildTime=${COMMIT_DATE}" \
      -o /out/promptsheond .
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/sachncs/promptsheon/backend.Version=${VERSION} -X github.com/sachncs/promptsheon/backend.Commit=${COMMIT} -X github.com/sachncs/promptsheon/backend.BuildTime=${COMMIT_DATE}" \
      -o /out/promptsheon .
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/sachncs/promptsheon/backend.Version=${VERSION} -X github.com/sachncs/promptsheon/backend.Commit=${COMMIT} -X github.com/sachncs/promptsheon/backend.BuildTime=${COMMIT_DATE}" \
      -o /out/promptsheon-healthcheck .

# ----- Runtime stage -------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -g 1000 promptsheon \
 && adduser -D -u 1000 -G promptsheon promptsheon

COPY --from=build /out/promptsheond /usr/local/bin/promptsheond
COPY --from=build /out/promptsheon /usr/local/bin/promptsheon
COPY --from=build /out/promptsheon-healthcheck /usr/local/bin/promptsheon-healthcheck

WORKDIR /data
RUN chown -R promptsheon:promptsheon /data

USER promptsheon

ENV PROMPTSHEON_ADDR=:8080 \
    PROMPTSHEON_DB_PATH=/data/promptsheon.db

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["promptsheond"]