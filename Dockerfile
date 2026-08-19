# Dockerfile
#
# Multi-stage build for the TypeScript port of Promptsheon.
#
# Stage 1 (deps): install pnpm and all workspace dependencies
# Stage 2 (build): tsc the server, next build the frontend
# Stage 3 (runtime): minimal Node 22 image with built artifacts
#
# Usage:
#   docker build -t promptsheon:dev .
#   docker run --rm -p 8080:8080 \
#     -v $(pwd)/data:/data \
#     -e PROMPTSHEON_DB_PATH=/data/promptsheon.db \
#     promptsheon:dev

# syntax=docker/dockerfile:1.7

# ----- Dependencies stage ---------------------------------------------------
FROM node:22-alpine AS deps
WORKDIR /src
RUN corepack enable
COPY packages/package.json packages/pnpm-workspace.yaml packages/pnpm-lock.yaml* ./
COPY packages/shared/package.json ./packages/shared/
COPY packages/server/package.json ./packages/server/
COPY frontend/package.json ./frontend/
RUN pnpm install --frozen-lockfile --ignore-scripts

# ----- Build stage ----------------------------------------------------------
FROM deps AS build
WORKDIR /src
COPY packages ./packages
COPY frontend ./frontend
COPY tsconfig.base.json* ./
RUN pnpm install --frozen-lockfile --ignore-scripts
RUN cd packages/shared && pnpm build 2>/dev/null || true
RUN cd packages/server && pnpm build
RUN cd frontend && pnpm build

# ----- Runtime stage --------------------------------------------------------
FROM node:22-alpine AS runtime
RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -g 1000 promptsheon \
  && adduser -D -u 1000 -G promptsheon promptsheon

WORKDIR /app
COPY --from=build /src/packages/server/package.json ./server/package.json
COPY --from=build /src/packages/server/dist ./server/dist
COPY --from=build /src/packages/shared/package.json ./shared/package.json
COPY --from=build /src/packages/shared/dist ./shared/dist
COPY --from=build /src/packages/shared/db ./shared/db
COPY --from=build /src/frontend/.next ./frontend/.next
COPY --from=build /src/frontend/public ./frontend/public
COPY --from=build /src/frontend/package.json ./frontend/package.json
COPY --from=build /src/node_modules ./node_modules

WORKDIR /data
RUN chown -R promptsheon:promptsheon /data /app

USER promptsheon

ENV PROMPTSHEON_PORT=8080 \
    PROMPTSHEON_HOST=0.0.0.0 \
    PROMPTSHEON_DB_PATH=/data/promptsheon.db \
    PROMPTSHEON_CAS_PATH=/data/.promptsheon

EXPOSE 8080
EXPOSE 3000
VOLUME ["/data"]

WORKDIR /app
CMD ["node", "server/dist/index.js"]
