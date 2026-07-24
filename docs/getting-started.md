# Getting Started

This walk-through takes a fresh checkout of Promptsheon from
`go build` to a fully working Capability that scores against a
Dataset. Time budget: ~10 minutes.

## 1. Build

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon
go build -o promptsheon-healthcheck ./cmd/promptsheon-healthcheck
```

Requirements: Go 1.26+ (see `go.mod`). v0.2.0 is SQLite-only by design; no
external database is required.

## 2. Configure environment

For a local single-node install, set:

```bash
export PROMPTSHEON_ADDR=":8080"
export PROMPTSHEON_AUTH=false          # local dev only; daemon refuses non-loopback binds with auth off
export PROMPTSHEON_LOG_LEVEL=info
export PROMPTSHEON_OPENAI_API_KEY="sk-..."   # at least one provider
```

See [docs/configuration.md](configuration.md) for the full
reference, including TLS, leader election, and OTel settings.

## 3. Start the daemon

```bash
./promptsheond
```

The daemon logs the bind address, migration count, and
plugin-manifest status on boot. The first run creates
`promptsheon.db` in the current directory.

## 4. Health probe

```bash
curl http://localhost:8080/health
# {"status":"healthy","version":"0.2.0","uptime":"..."}

curl http://localhost:8080/ready
# {"status":"ready","go":"go1.26.x","database":"ok"}
```

Kubernetes-style aliases: `GET /livez` (liveness) and `GET /readyz`
(readiness) return the same payload as `/health` and `/ready`.

## 5. Create a Capability

A Capability lives in a Workspace → Project → Capability tree.

```bash
# Create a Workspace.
curl -X POST http://localhost:8080/api/v1/workspaces \
  -H 'Content-Type: application/json' \
  -d '{"name":"acme"}'
# {"id":"w1", ...}

# Create a Project under that Workspace.
curl -X POST http://localhost:8080/api/v1/workspaces/w1/projects \
  -H 'Content-Type: application/json' \
  -d '{"name":"summariser"}'
# {"id":"p1", ...}

# Create a Capability under that Project.
curl -X POST http://localhost:8080/api/v1/projects/p1/capabilities \
  -H 'Content-Type: application/json' \
  -d '{"name":"summariser","description":"Summarise long docs"}'
# {"id":"c1", ...}
```

## 6. Add an immutable Version

Versions are immutable builds of a Capability's Manifest. The
Manifest references content-addressed artifacts by `(kind, hash)`;
each leaf is the SHA-256 of the artifact's canonical encoding
(see [docs/algorithms.md](algorithms.md#content-addressed-storage)).

```bash
# The five required leaves share the placeholder SampleManifestHash
# for the dev walk-through. Production wires a real CAS push before
# the version is created.
SAMPLE="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

curl -X POST http://localhost:8080/api/v1/capabilities/c1/versions \
  -H 'Content-Type: application/json' \
  -d "{
    \"version\": 1,
    \"manifest\": {
      \"prompt\":          {\"kind\":\"prompt\",          \"hash\":\"$SAMPLE\"},
      \"model_policy\":    {\"kind\":\"model_policy\",    \"hash\":\"$SAMPLE\"},
      \"runtime_policy\":  {\"kind\":\"runtime_policy\",  \"hash\":\"$SAMPLE\"},
      \"context_contract\":{\"kind\":\"context\",         \"hash\":\"$SAMPLE\"},
      \"memory\":          {\"kind\":\"memory\",          \"hash\":\"$SAMPLE\"}
    }
  }"
# {"id":"v1", ...}
```

## 7. Drive the Release lifecycle

A Release binds a Version to a tenant Environment. The
MakerChecker approval policy requires a non-creator identity
to vote before Activate succeeds.

```bash
# 1. Create a Pending Release (as "alice").
REL=$(curl -sS -X POST http://localhost:8080/api/v1/versions/v1/releases \
        -H 'Content-Type: application/json' \
        -H 'X-Promptsheon-User: alice' \
        -d '{"environment":"prod"}' | jq -r .id)

# 2. A non-creator identity ("bob") casts an Approve vote.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/votes \
     -H 'Content-Type: application/json' \
     -H 'X-Promptsheon-User: bob' \
     -d '{"identity":"bob","decision":"approve"}'

# 3. Activate (consults MakerChecker; 409 if preconditions fail
#    or quorum is not satisfied).
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/activate \
     -H 'X-Promptsheon-User: alice'

# 4. Invoke through the configured LLM provider. The Release
#    decides provider + model; the request carries only the inputs.
curl -sS -X POST http://localhost:8080/api/v1/releases/$REL/invoke \
     -H 'Content-Type: application/json' \
     -H 'X-Promptsheon-User: alice' \
     -d '{"inputs":{"q":"hello"}}'
# {"id":"<exec-id>", "model":"claude-...", "prompt_tokens":1, "output_tokens":42, "cost_usd":0.0014, ...}
```

## 8. Wire the harness loop

The harness-engineering primitive — Datasets, Preconditions,
EvalRuns — closes the iteration loop on a Capability.

```bash
# Add a dataset (cases.json is an array of {inputs, expected, description}).
curl -X POST http://localhost:8080/api/v1/capabilities/c1/datasets \
  -H 'Content-Type: application/json' \
  -d @cases.json

# Add a precondition that gates Activate.
curl -X POST http://localhost:8080/api/v1/capabilities/c1/preconditions \
  -H 'Content-Type: application/json' \
  -d '{"name":"go-test","command":"go test ./...","timeout_sec":60}'

# Drive the same Release lifecycle, but now Activate will run
# the precondition and return 409 on failure.
```

See [docs/harness.md](harness.md) and [docs/eval.md](eval.md) for
the full surface.

## 9. Tail the metrics and audit chain

```bash
# Prometheus scrape.
curl http://localhost:8080/metrics | grep promptsheon_

# Audit chain (admin-only).
curl 'http://localhost:8080/api/v1/audit?user_id=alice&limit=20' \
  -H 'X-Promptsheon-User: admin'

# Verify the chain (admin-only; returns LastRowID, LastHash, etc.).
curl http://localhost:8080/api/v1/audit/verify \
  -H 'X-Promptsheon-User: admin'
```

See [docs/observability.md](observability.md) and
[docs/slos.md](slos.md) for the metric inventory and the three
first-class SLOs.

## 10. Tear down

```bash
rm -f promptsheon.db promptsheon.db-shm promptsheon.db-wal
```

The next `./promptsheond` boots from a fresh state.

---

## Troubleshooting

- **`./promptsheond` exits with "PROMPTSHEON_AUTH=false is only
  valid for loopback binds"** — you're binding a non-loopback
  address with auth disabled. Set `PROMPTSHEON_AUTH=true`, or
  bind to `127.0.0.1`, or set `PROMPTSHEON_TLS_CERT_FILE` +
  `PROMPTSHEON_TLS_KEY_FILE`. See [docs/security.md](security.md).
- **Activate returns 409 even after a vote** — the MakerChecker
  policy requires a non-creator Approve. The vote was cast by
  the same identity as the Release's CreatedBy. Cast a second
  vote as a different user.
- **`/metrics` returns 401** — the scrape endpoint is
  `PermAuditRead`-gated. Either pass an admin API key in
  `Authorization: Bearer ...`, or bind the daemon to a loopback
  listener and rely on network controls.
- **No traces in your OTel collector** — set
  `PROMPTSHEON_OTEL_ENDPOINT=otel-collector:4317` and (optionally)
  `PROMPTSHEON_OTEL_SAMPLE_RATIO=0.1` for high-volume production.

See [docs/troubleshooting.md](troubleshooting.md) for the full
checklist.