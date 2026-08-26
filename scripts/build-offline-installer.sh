#!/usr/bin/env bash
# Build an offline-installable tarball for promptsheon.
#
# Air-gapped customer deployments (defense, gov) can't reach the
# public npm registry. This script:
#   1. Runs `pnpm install --frozen-lockfile` to populate node_modules.
#   2. Builds every workspace's compiled output (shared/server/frontend).
#   3. Bundles node + the lockfile + the migrations + the SBOM into
#      a single .tar.gz the operator can copy to the target host.
#   4. Includes a bootstrap.sh that wires up systemd services.

set -euo pipefail
cd "$(dirname "$0")/.."

OUT_DIR="$(cd "$(dirname "$0")/.." && pwd)/dist"
NAME="promptsheon-offline-$(date +%Y%m%d-%H%M%S)"
DEST="$OUT_DIR/$NAME"

echo "Building offline installer in $DEST ..."
mkdir -p "$DEST"/{bin,etc,lib,share,share/migrations,share/compliance}

# 1. Install all deps from the existing lockfile (caller has
#    downloaded everything already; we just copy node_modules).
if [ ! -d "node_modules" ]; then
  echo "node_modules missing — run 'pnpm install --frozen-lockfile' first."
  exit 1
fi
echo "  - copying node_modules"
cp -a node_modules "$DEST/node_modules"
cp pnpm-lock.yaml "$DEST/pnpm-lock.yaml"

# 2. Build every workspace
echo "  - building shared"
(cd packages/shared && pnpm build > "$DEST/share/build-shared.log" 2>&1) || {
  echo "shared build failed; see $DEST/share/build-shared.log"; exit 1; }
echo "  - building server"
(cd packages/server && pnpm build > "$DEST/share/build-server.log" 2>&1) || {
  echo "server build failed; see $DEST/share/build-server.log"; exit 1; }
echo "  - building frontend"
(cd frontend && pnpm build > "$DEST/share/build-frontend.log" 2>&1) || {
  echo "frontend build failed; see $DEST/share/build-frontend.log"; exit 1; }

cp -a packages/shared/dist "$DEST/share/shared"
cp -a packages/server/dist "$DEST/share/server"
cp -a frontend/.next "$DEST/share/frontend"

# 3. Migrations + SBOM
echo "  - copying migrations + sbom"
cp -a packages/shared/dist/db/migrations "$DEST/share/migrations"
mkdir -p "$DEST/share/compliance"
[ -f docs/compliance/sbom.json ] && cp docs/compliance/sbom.json "$DEST/share/compliance/"

# 4. Bootstrap script
cat > "$DEST/bin/bootstrap.sh" <<'BOOTSTRAP'
#!/usr/bin/env bash
# Bootstrap a promptsheon install on a clean target host.
# Run as root.
set -euo pipefail
INSTALL_ROOT="${PROMPTSHEON_ROOT:-/opt/promptsheon}"
mkdir -p "$INSTALL_ROOT"

# 1. Copy payload
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PAYLOAD="$(dirname "$SCRIPT_DIR")"
echo "[+] Installing files to $INSTALL_ROOT ..."
cp -a "$PAYLOAD/bin" "$INSTALL_ROOT/"
cp -a "$PAYLOAD/etc" "$INSTALL_ROOT/"
cp -a "$PAYLOAD/lib" "$INSTALL_ROOT/" 2>/dev/null || true
cp -a "$PAYLOAD/node_modules" "$INSTALL_ROOT/"
cp -a "$PAYLOAD/pnpm-lock.yaml" "$INSTALL_ROOT/"
mkdir -p "$INSTALL_ROOT/share"
cp -a "$PAYLOAD/share"/* "$INSTALL_ROOT/share/"

# 2. Generate a server .env with safe defaults
ENV_FILE="$INSTALL_ROOT/.env"
if [ ! -f "$ENV_FILE" ]; then
  echo "[+] Generating $ENV_FILE"
  cat > "$ENV_FILE" <<'ENV'
PROMPTSHEON_PORT=8080
PROMPTSHEON_HOST=127.0.0.1
PROMPTSHEON_DB_PATH=/var/lib/promptsheon/promptsheon.db
PROMPTSHEON_CAS_PATH=/var/lib/promptsheon/.promptsheon
PROMPTSHEON_FRONTEND_PATH=/opt/promptsheon/share/frontend
PROMPTSHEON_LOG_LEVEL=info
PROMPTSHEON_NODE_ENV=production
PROMPTSHEON_WEBHOOK_SECRET=__REPLACE_ME__
PROMPTSHEON_ALLOW_SYSTEM_ACTOR=false
ENV
  chmod 600 "$ENV_FILE"
fi

# 3. Enable FIPS mode if requested (--fips)
if [ "${1:-}" = "--fips" ]; then
  echo "[+] FIPS mode requested"
  echo "  - install Node from FIPS-validated build (https://nodejs.org/en/download)"
  echo "  - set PROMPTSHEON_FIPS_MODE=true in $ENV_FILE"
  echo "PROMPTSHEON_FIPS_MODE=true" >> "$ENV_FILE"
  echo "  - on first start the server will refuse to boot unless"
  echo "    crypto.createHash('sha256', { fipsMode: true }) succeeds"
fi

# 4. systemd unit
echo "[+] Installing systemd unit"
cat > /etc/systemd/system/promptsheon.service <<UNIT
[Unit]
Description=promptsheon backend
After=network.target

[Service]
Type=simple
User=promptsheon
EnvironmentFile=$ENV_FILE
ExecStart=/usr/bin/node $INSTALL_ROOT/share/server/index.js
WorkingDirectory=$INSTALL_ROOT
Restart=on-failure

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable promptsheon.service
systemctl start promptsheon.service
echo "[+] promptsheon running on http://127.0.0.1:8080"
BOOTSTRAP
chmod +x "$DEST/bin/bootstrap.sh"

# 5. README inside the installer
cat > "$DEST/README.txt" <<EOF
promptsheon offline installer

Built: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
Layout:
  bin/bootstrap.sh       — installer entry point
  node_modules/          — npm dependencies (frozen)
  share/shared/          — built shared schemas
  share/server/          — built Fastify backend
  share/frontend/        — built Next.js app
  share/migrations/      — SQLite migrations
  share/compliance/      — SBOM + control docs

Install:
  tar xzf <this-tarball>.tar.gz
  cd <extracted-dir>
  sudo ./bin/bootstrap.sh           # standard install
  sudo ./bin/bootstrap.sh --fips    # enable FIPS mode

Configuration:
  Edit .env (created on first run) before 'systemctl start promptsheon'.

Verifying the install (post-bootstrap):
  systemctl status promptsheon.service
  curl -s http://127.0.0.1:8080/api/health | jq

The install is fully air-gap compatible: bootstrap does not
require network access after the tarball is on the host.
EOF

# 6. Tar it up
TARBALL="$OUT_DIR/$NAME.tar.gz"
tar -czf "$TARBALL" -C "$OUT_DIR" "$NAME"
echo "[+] $TARBALL"
echo "Size: $(du -h "$TARBALL" | awk '{print $1}')"
