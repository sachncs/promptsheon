# Promptsheon

The Open-Source Control Plane for AI Capabilities.

This directory holds the production deployment artefacts:

- `deploy/helm/promptsheon/` — Helm chart for k8s deployment
- `Dockerfile` — multi-stage build for the daemon image
- `Dockerfile.cli` (recommended addition) — slim CLI image

The chart defaults to a single-replica deployment with an
8Gi PVC-backed SQLite database. Production deployments should
switch to Postgres:

```sh
helm install promptsheon deploy/helm/promptsheon \
  --set config.dbBackend=postgres \
  --set config.dbDSN="postgres://promptsheon:CHANGE_ME@postgres:5432/promptsheon?sslmode=require"
```

The Postgres RLS migrations in
`internal/store/migrations/postgres/100_rls.sql` enable per-
workspace row-level security automatically on first boot.
