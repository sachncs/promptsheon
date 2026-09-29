# Phase 0 baseline evidence

This directory records the evidence used to start the production hardening
program. It is intentionally versioned so later phases can compare changes
against a known baseline.

- [Capability matrix](capability-matrix.md)
- [Repository inventory](inventory.json)
- [Reliability and throughput baseline](reliability.md)

Refresh this evidence when a supported surface, runtime, deployment contract,
or quality gate changes.

Regenerate the inventory with:

```bash
pnpm inventory:generate
```
