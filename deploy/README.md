# Helm chart for Promptsheon.

This chart deploys a single-replica `promptsheond` daemon with a
PVC-backed SQLite database. Vector search is provided by the
[sqlite-vec](https://github.com/asg017/sqlite-vec) extension loaded
on startup.

## Constraints

- **Single replica.** SQLite is single-writer; `values.schema.json`
  rejects `replicaCount>1`.
- **SQLite only.** Postgres support was removed; the chart no longer
  accepts `dbBackend=postgres`.
- **Auth default `true`.** Setting `auth=false` requires the
  `insecureLoopback=true` opt-in (the schema enforces this).

## Required values

```yaml
replicaCount: 1
config:
  auth: "true"
vault:
  enabled: true
  key: "<32-byte hex from your secrets store>"
```

The vault key Secret is rendered only when `vault.enabled=true`.
Plaintext DSNs and other credentials never appear in the ConfigMap.

## Uninstall

The PVC carries `helm.sh/resource-policy: keep` so the data volume
outlives `helm uninstall`. Delete it explicitly only after a confirmed
backup.
