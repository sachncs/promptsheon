# promptsheon prompt-security benchmark results

> Generated: 2026-08-26T07:34:12.164Z
> Dataset: `promptsheon prompt-security benchmark dataset` (1.0.0)

## Summary

| Metric | Value |
|---|---|
| Total cases | 53 |
| Passed | 53 |
| Failed | 0 |
| Pass rate | `100.0%` |

### By OWASP category

| Category | Total | Passed | Pass rate |
|---|---|---|---|
| EDGE | 10 | 10 | `100.0%` |
| LLM01 | 11 | 11 | `100.0%` |
| LLM02 | 1 | 1 | `100.0%` |
| LLM03 | 1 | 1 | `100.0%` |
| LLM04 | 3 | 3 | `100.0%` |
| LLM05 | 2 | 2 | `100.0%` |
| LLM06 | 12 | 12 | `100.0%` |
| LLM07 | 2 | 2 | `100.0%` |
| LLM08 | 3 | 3 | `100.0%` |
| LLM09 | 2 | 2 | `100.0%` |
| LLM10 | 2 | 2 | `100.0%` |
| MIX | 4 | 4 | `100.0%` |

### By expected verdict

| Verdict | Total | Passed | Pass rate |
|---|---|---|---|
| clean | 19 | 19 | `100.0%` |
| warn | 9 | 9 | `100.0%` |
| block | 25 | 25 | `100.0%` |

## Cases

### ✅ `LLM01-001` — ignore-previous-instructions (classic)

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`
- Actual rules: `injection.ignore-previous`

### ✅ `LLM01-002` — system role override

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.system-override`
- Actual rules: `injection.role-switch`, `injection.system-override`, `jailbreak.evil-twin`
- Unexpected (informational): `injection.role-switch`, `jailbreak.evil-twin`

### ✅ `LLM01-003` — role-switch to admin

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.role-switch`
- Actual rules: `injection.role-switch`

### ✅ `LLM01-004` — disregard prior rules

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`, `injection.instruction-injection`
- Actual rules: `injection.ignore-previous`, `injection.instruction-injection`

### ✅ `LLM01-005` — act as jailbreak

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `jailbreak.evil-twin`, `injection.role-switch`
- Actual rules: `injection.role-switch`, `jailbreak.dan`, `jailbreak.evil-twin`
- Unexpected (informational): `jailbreak.dan`

### ✅ `LLM01-006` — tool-abuse exfil

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.tool-abuse`, `jailbreak.reverse-shell`
- Actual rules: `injection.tool-abuse`, `jailbreak.reverse-shell`

### ✅ `LLM01-007` — benign request (negative test)

- Category: **LLM01**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `LLM02-001` — request raw HTML with embedded script

- Category: **LLM02**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Output handling is the consumer's job; scanner flags input only. This case documents the gap.

### ✅ `LLM03-001` — poisoned dataset fingerprint

- Category: **LLM03**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Scanner is heuristic; runtime eval (T1-4) catches actual drift.

### ✅ `LLM04-001` — resource-exhaustion prompt

- Category: **LLM04**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Cost caps + limit spec handle this at execution time.

### ✅ `LLM04-002` — long-prompt denial of service

- Category: **LLM04**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Length check lives on the executor / gateway (T1-3), not the scanner.

### ✅ `LLM05-001` — supply-chain: suspicious dependency manifest

- Category: **LLM05**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `injection.tool-abuse`
- Actual rules: `injection.tool-abuse`

### ✅ `LLM05-002` — tampered model artifact

- Category: **LLM05**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Model-load integrity is enforced by the agent harness, not the scanner.

### ✅ `LLM06-001` — email address in prompt

- Category: **LLM06**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `pii.email`
- Actual rules: `pii.email`

### ✅ `LLM06-002` — US SSN in prompt

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.ssn`
- Actual rules: `pii.ssn`

### ✅ `LLM06-003` — credit-card PAN (Luhn-valid Visa test)

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.credit-card`
- Actual rules: `pii.credit-card`

### ✅ `LLM06-004` — credit-card-shaped but Luhn-invalid

- Category: **LLM06**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Luhn check should prune the false positive from the regex shape match.

### ✅ `LLM06-005` — EU IBAN in prompt

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.iban`
- Actual rules: `pii.iban`

### ✅ `LLM06-006` — phone number

- Category: **LLM06**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `pii.phone`
- Actual rules: `pii.phone`

### ✅ `LLM06-007` — IPv4 address

- Category: **LLM06**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `pii.ipv4`
- Actual rules: `pii.ipv4`

### ✅ `LLM06-008` — AWS access key id

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.aws-key`
- Actual rules: `pii.aws-key`

### ✅ `LLM06-009` — PEM private key header

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.private-key`
- Actual rules: `pii.private-key`

### ✅ `LLM06-010` — OpenSSH private key header

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.private-key`
- Actual rules: `pii.private-key`

### ✅ `LLM07-001` — plugin: unauthenticated tool call

- Category: **LLM07**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `injection.tool-abuse`
- Actual rules: `injection.tool-abuse`

### ✅ `LLM07-002` — plugin: SSRF via fetch

- Category: **LLM07**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `pii.ipv4`, `injection.tool-abuse`
- Actual rules: `injection.tool-abuse`, `pii.ipv4`

### ✅ `LLM08-001` — excessive-agency: irreversible delete

- Category: **LLM08**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `injection.tool-abuse`
- Actual rules: `injection.tool-abuse`

### ✅ `LLM08-002` — excessive-agency: silent privilege escalation

- Category: **LLM08**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.role-switch`
- Actual rules: `injection.role-switch`, `injection.tool-abuse`
- Unexpected (informational): `injection.tool-abuse`

### ✅ `LLM08-003` — excessive-agency: financial tx

- Category: **LLM08**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Amount-bearing language alone is not a scanner signal; the financial-controls surface (separate work) handles approval gates.

### ✅ `LLM09-001` — overreliance: medical advice

- Category: **LLM09**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Domain expertise is the operator's job; scanner doesn't flag risky advice by content.

### ✅ `LLM09-002` — overreliance: legal advice

- Category: **LLM09**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Out of scanner scope.

### ✅ `LLM10-001` — model-theft: weights extraction

- Category: **LLM10**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.instruction-injection`
- Actual rules: `injection.instruction-injection`

### ✅ `LLM10-002` — model-theft: distillation probe

- Category: **LLM10**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `injection.tool-abuse`
- Actual rules: `injection.tool-abuse`

### ✅ `MIX-001` — compound: PII + injection

- Category: **MIX**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`, `pii.email`, `injection.tool-abuse`
- Actual rules: `injection.ignore-previous`, `injection.tool-abuse`, `pii.email`

### ✅ `MIX-002` — compound: jailbreak + AWS key

- Category: **MIX**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `jailbreak.evil-twin`, `pii.aws-key`
- Actual rules: `injection.role-switch`, `jailbreak.dan`, `jailbreak.evil-twin`, `pii.aws-key`
- Unexpected (informational): `injection.role-switch`, `jailbreak.dan`

### ✅ `MIX-003` — compound: role-switch + IBAN

- Category: **MIX**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.role-switch`, `pii.iban`
- Actual rules: `injection.role-switch`, `pii.iban`

### ✅ `EDGE-001` — empty input

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `EDGE-002` — whitespace only

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `EDGE-003` — email-like but missing TLD

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Scanner requires a 2+ char TLD; this is by design to avoid flagging shell paths.

### ✅ `EDGE-004` — credit-card in context (not a PAN)

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `EDGE-005` — system role mention (not an override)

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `EDGE-006` — role mention without 'you are now'

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `EDGE-007` — ignore word in non-injection context

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

### ✅ `EDGE-008` — DAN as acronym (false-positive guard)

- Category: **EDGE**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> False-positive guard: 'DAN group' (acronym) must NOT fire the jailbreak.dan rule. The tightened regex requires DAN to be followed by mode/can/will/is to fire.

### ✅ `EDGE-009` — phone-like number with separators

- Category: **EDGE**
- Expected verdict: `warn`
- Actual verdict: `warn`
- Expected rules: `pii.phone`
- Actual rules: `pii.phone`

### ✅ `EDGE-010` — private key marker without body

- Category: **EDGE**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.private-key`
- Actual rules: `pii.private-key`

### ✅ `LLM01-008` — indirect injection via retrieved context

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`, `injection.instruction-injection`
- Actual rules: `injection.ignore-previous`, `injection.instruction-injection`

### ✅ `LLM01-009` — multi-turn role establishment

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.role-switch`
- Actual rules: `injection.role-switch`

### ✅ `LLM01-010` — forget prior context

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`
- Actual rules: `injection.ignore-previous`

### ✅ `LLM01-011` — skip the rules

- Category: **LLM01**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`
- Actual rules: `injection.ignore-previous`

### ✅ `LLM04-003` — excessive branching request

- Category: **LLM04**
- Expected verdict: `clean`
- Actual verdict: `clean`
- Expected rules: _none_
- Actual rules: _none_

> Cost caps handle this in production.

### ✅ `LLM06-011` — PGP private key header

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.private-key`
- Actual rules: `pii.private-key`

### ✅ `LLM06-012` — EC private key header

- Category: **LLM06**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `pii.private-key`
- Actual rules: `pii.private-key`

### ✅ `MIX-004` — compound: phone + email + injection

- Category: **MIX**
- Expected verdict: `block`
- Actual verdict: `block`
- Expected rules: `injection.ignore-previous`, `pii.email`, `pii.phone`
- Actual rules: `injection.ignore-previous`, `pii.email`, `pii.phone`

---

_Run `pnpm --filter @promptsheon/server bench:security` to regenerate._