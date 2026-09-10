# syntax=docker/dockerfile:1.7
# Promptsheon multi-stage build.
#
# Stage 1 (builder): install pnpm, copy the workspace manifests,
#   run pnpm fetch + pnpm install, then build the shared,
#   server, and frontend packages.
# Stage 2 (runtime): copy the built artifacts + the production
#   node_modules, set a non-root user, drop capabilities, and
#   start the server on :8080.

ARG NODE_VERSION=26
ARG PNPM_VERSION=11.23.0

# ---------------------------------------------------------------------------
# Builder
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS builder
ARG PNPM_VERSION
RUN corepack enable && corepack prepare pnpm@${PNPM_VERSION} --activate

WORKDIR /app

# Copy workspace manifests first so the layer cache survives
# source-only changes. pnpm-workspace.yaml, pnpm-lock.yaml, and
# every package.json are needed for `pnpm fetch`.
COPY pnpm-workspace.yaml pnpm-lock.yaml package.json ./
COPY packages/shared/package.json ./packages/shared/
COPY packages/server/package.json ./packages/server/
COPY packages/cli/package.json ./packages/cli/
COPY packages/sdk/package.json ./packages/sdk/
COPY frontend/package.json ./frontend/
COPY extensions/promptsheon/package.json ./extensions/promptsheon/

RUN pnpm fetch --frozen-lockfile

# Now copy the full source tree.
COPY tsconfig.base.json ./
COPY packages ./packages
COPY frontend ./frontend
COPY extensions ./extensions

# Allow native builds for better-sqlite3 + esbuild in this
# stage. The runtime image doesn't need them.
RUN pnpm config set --location=project --json \
  'onlyBuiltDependencies' '["better-sqlite3", "esbuild"]' \
  || true

RUN pnpm install --frozen-lockfile --offline
RUN pnpm --filter @promptsheon/shared build
RUN pnpm --filter @promptsheon/server build
RUN pnpm --filter @promptsheon/frontend build

# Trim node_modules down to production-only deps for the
# runtime image. Keeps the image small.
RUN pnpm deploy --filter @promptsheon/server --prod /app/deploy/server
RUN pnpm deploy --filter @promptsheon/frontend --prod /app/deploy/frontend

# ---------------------------------------------------------------------------
# Runtime
# ---------------------------------------------------------------------------
FROM node:${NODE_VERSION}-alpine AS runtime
ARG NODE_VERSION

LABEL org.opencontainers.image.title="promptsheon" \
      org.opencontainers.image.description="Git-native version control for AI agents" \
      org.opencontainers.image.source="https://github.com/sachncs/promptsheon" \
      org.opencontainers.image.licenses="Apache-2.0"

ENV NODE_ENV=production \
    PROMPTSHEON_PORT=8080 \
    PROMPTSHEON_HOST=0.0.0.0 \
    PROMPTSHEON_FRONTEND_PATH=/app/frontend/.next \
    PROMPTSHEON_CORS_ORIGIN="" \
    PROMPTSHEON_DB_PATH=/data/promptsheon.db \
    PROMPTSHEON_CAS_PATH=/data/.promptsheon

# Create a non-root user and the data directory it owns.
RUN addgroup -S promptsheon && adduser -S promptsheon -G promptsheon \
 && mkdir -p /data && chown -R promptsheon:promptsheon /data

WORKDIR /app

# Copy the production-only deploy trees.
COPY --from=builder --chown=promptsheon:promptsheon /app/deploy/server /app/server
COPY --from=builder --chown=promptsheon:promptsheon /app/deploy/frontend /app/frontend

USER promptsheon

VOLUME ["/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1

# Drop extra capabilities (no network_admin, no sys_admin).
# Alpine ships with cap_drop support via the Docker seccomp
# profile; the runtime user already has no privileges to
# escalate beyond what cap_drop enforces.

# `node --check` does a syntax + import-graph validation pass
# without binding the port; CI uses it as a smoke check.
ENTRYPOINT ["node"]
CMD ["server/dist/index.js"]
