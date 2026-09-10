---
layout: page
title: Installation
subtitle: Install Promptsheon on a developer laptop, a server, or a container.
---

# Installation

Promptsheon runs on Node.js 26 and pnpm 11. You can install it from source, from a Docker image, or from the offline installer tarball for air-gapped deployments.

## Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Node.js | 26 or newer | Required for `better-sqlite3` native build |
| pnpm | 11.x | `corepack enable && corepack prepare pnpm@11 --activate` |
| Disk | 1 GB minimum | For the database, the CAS, and the build output |
| RAM | 512 MB minimum | 2 GB recommended for parallel eval runs |

A C toolchain (`gcc`, `make`, `python3`) is required for the `better-sqlite3` native build. On Debian/Ubuntu, install `build-essential` and `python3`. On macOS, install Xcode Command Line Tools with `xcode-select --install`.

## Option 1 — From source (recommended)

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
pnpm install
cp .env.example .env
$EDITOR .env
pnpm dev
```

The `pnpm dev` script launches the backend (`:8080`) and the frontend (`:3000`) together with hot reload. This is the recommended workflow during development.

To build for production:

```bash
pnpm typecheck
pnpm --filter @promptsheon/server build
pnpm --filter @promptsheon/frontend build
pnpm start
```

## Option 2 — Docker

A multi-stage Dockerfile ships at the repo root. Build and run:

```bash
docker build -t promptsheon:latest .
docker run --rm -p 8080:8080 \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -v $PWD/.promptsheon:/app/.promptsheon \
  promptsheon:latest
```

The image bundles the pnpm workspace, the shared package, the server, and the Next.js frontend build output. It listens on `:8080` and serves the UI from the same port.

The image runs as a non-root user, drops capabilities, and exposes a read-only root filesystem. The audit chain and SQLite database live on the volume you mount at `/app/.promptsheon`.

## Option 3 — Offline installer (air-gapped)

Government, defense, and regulated customers run on hosts with no outbound internet. The repo ships an offline installer that bundles every dependency, the SBOM, and a `systemd` bootstrap:

```bash
bash scripts/build-offline-installer.sh    # build the tarball
sudo bash bin/bootstrap.sh --fips          # install + FIPS mode
```

The step-by-step runbook lives at [`docs/operations/air-gap-rhel.md`](https://github.com/sachncs/promptsheon/blob/master/docs/operations/air-gap-rhel.md) and covers pre-flight, FIPS-mode requirements, upgrades, backups, DR, and the FIPS gate's refuse-to-boot contract.

## Verify the install

In a second terminal, run:

```bash
curl http://localhost:8080/api/health
# {"status":"ok","service":"promptsheon-server", …}
```

Open `http://localhost:3000` (or `http://localhost:8080` on Docker) in a browser and complete the onboarding wizard.

## Upgrade

```bash
git pull
pnpm install
pnpm --filter @promptsheon/shared build
pnpm --filter @promptsheon/server build
pnpm --filter @promptsheon/frontend build
pnpm start
```

For air-gapped deployments, see the upgrade section of the [air-gap runbook](https://github.com/sachncs/promptsheon/blob/master/docs/operations/air-gap-rhel.md).

## Uninstall

The install leaves no system state outside the directory you mounted as `/app/.promptsheon`. To uninstall:

1. Stop the server (`Ctrl+C` or `systemctl stop promptsheon`).
2. Delete the install directory.
3. Delete the volume (`.promptsheon/` or the Docker mount).

## Where to go next

| You want to… | Read this |
|--------------|-----------|
| Run your first release | [Getting started]({{ '/getting-started/' | relative_url }}) |
| Tune environment variables | [Configuration]({{ '/configuration/' | relative_url }}) |
| Understand the model | [Core concepts]({{ '/core-concepts/' | relative_url }}) |
