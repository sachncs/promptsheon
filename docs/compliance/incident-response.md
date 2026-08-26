# Incident response — promptsheon

Complements the pen-test plan (`pen-test-plan.md`). This doc
covers the **what** of our IR posture; the **how** lives in the
private IR-Playbook repo (out of scope for this public artefact).

## Severity classification

| Class | Examples | First response | Public statement |
|---|---|---|---|
| SEV-1 (Critical) | Auth bypass, RCE, audit-chain tampering, vault leak, signing-key leak | 1 hour | Within 24 hours (per SOC 2 + GDPR) |
| SEV-2 (High) | Privilege escalation, data corruption, prompt-injection exec | 4 hours | Within 72 hours |
| SEV-3 (Medium) | DoS via gateway abuse, rate-limit bypass | 1 business day | Optional |
| SEV-4 (Low) | Cosmetic / docs / non-data UX | 1 business week | None |

## Detection sources

1. CI failure on `master` — every PR runs typecheck + tests; a
   merge of a failing PR is auto-reverted.
2. Audit-chain verification scheduled job — flips a
   `tamper_detected=true` flag in `audit_chain_state`.
3. Rate-limiter saturation alert — every N consecutive 429s
   in a window fires a pager.
4. Vendor security advisories — dependabot + open-source CVE
   feeds; promptsheon SRE on-call rotates.

## First-responder checklist (SEV-1)

1. **Page the on-call** via the configured pager.
2. **Open the audit-chain verify endpoint** — `/api/audit/verify`.
   If `valid=false`, snapshot the broken row id and stop:
   *do not write* until the forensic review is complete.
3. **Rotate signing keys + vault keys** for any org whose data
   is suspected. The Vault KMS abstraction (`/api/vault/rotate`)
   generates a fresh key and re-encrypts every secret under it.
4. **Snapshot the database** for forensic capture — sqlite hot
   backup + WAL snapshot.
5. **Open the post-mortem doc** (template in `/templates/`),
   assign an incident commander, schedule a post-mortem
   within 5 business days.

## Customer communication

For SEV-1: written notice within 24 hours via the customer
success mailing list, regardless of whether customer data was
actually exposed. SEV-2: notice within 72 hours, conditional on
data exposure. SEV-3 / SEV-4: bundled into the monthly customer
newsletter.

## Customer-side obligations

Customers running promptsheon self-hosted are the data
controller for their own tenants. Their IR runbook lives in
their private docs; promptsheon's IR support is on-call during
business hours and 24/7 for SEV-1 tenants (response under 1
hour, 24/7).

## Coordination with the LLM provider

When an SEV-1 involves the upstream LLM (OpenAI / Anthropic /
Bedrock), promptsheon SRE coordinates with the vendor's security
team via the contacts in `vendor-risk-questionnaire.md`. Customer
data flow to the LLM is documented in `docs/security/data-flow.md`.

## Tabletop exercise

Quarterly IR tabletop with SRE + eng + customer-success. One
scenario per quarter, picked from a rotating menu (audit-chain
tampering, vault leak, signing-key compromise, gateway abuse,
etc.). Lessons feed into the IR playbook.

## Post-mortem format

Every SEV-1 + SEV-2 produces a blameless post-mortem published
internally within 5 business days. Customer-impacting findings
are summarised (with redacted details if any) in the next customer
newsletter.

## Rehearsal of the runbook

Once per quarter, SRE walks the full first-responder checklist
in a staging environment with a synthetic incident. Time-to-first-
action is tracked and trended.

## What is NOT in this document

- Contact lists (private to the SRE team).
- Customer-specific access history (lives in the customer's SIEM).
- Vendor-specific IR runbooks (the vendor publishes their own).
- Legal / disclosure templates (private to the legal team).
