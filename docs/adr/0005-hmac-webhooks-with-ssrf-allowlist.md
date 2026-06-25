# ADR 0005: Sign webhooks with HMAC and refuse private destinations by default

- **Status:** Accepted
- **Date:** 2024-01-01
- **Supersedes:** —
- **Superseded by:** —

## Context

Webhooks are an attacker-controllable request surface. Two distinct classes of attack apply:

1. **Forgery.** A third party who can sniff or guess the destination URL and the event payload can deliver fake events.
2. **SSRF (Server-Side Request Forgery).** A third party who can register a webhook destination can cause our server to make an outbound HTTP request to an internal address (loopback, RFC1918, link-local), opening an indirect channel for further attacks.

## Decision

We address forgery with HMAC-SHA256. Every webhook delivery includes an `X-Promptsheon-Signature` header that is `HMAC-SHA256(secret, body)`. The secret is per-endpoint, generated at registration time, and returned to the user once. Receivers verify the signature before trusting the payload.

We address SSRF by validating destination URLs at registration and at every delivery. The default policy refuses:

- Loopback (`127.0.0.0/8`, `::1`)
- Link-local (`169.254.0.0/16`, `fe80::/10`)
- Private (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`)
- Multicast, unspecified, and a small number of other reserved ranges

The policy can be relaxed for local development by setting `PROMPTSHEON_WEBHOOK_ALLOW_PRIVATE=true`. The flag is intended for `localhost` testing only and is explicitly logged at server start.

## Options considered

1. **HMAC + SSRF allowlist (chosen).** Strong forgery protection, conservative default. Developers can opt out.
2. **mTLS.** Strongest authentication, but requires the receiver to present a certificate. Most webhook receivers cannot.
3. **Signed JWT in a query parameter.** Workable, but JWT in URLs is awkward (length, logging) and not obviously stronger than HMAC for this use case.
4. **No SSRF protection.** Untenable for any deployment that has access to internal services.

## Consequences

Positive:

- Receivers can verify the authenticity of every delivery with one HMAC compute.
- The default SSRF policy blocks the most common attacks out of the box. The opt-in flag is loud (logged) so the operational cost of disabling it is visible.
- The secret is generated with `crypto/rand` and is 32 bytes hex-encoded by default.

Negative:

- Receivers that want to consume the secret programmatically must store it somewhere. The secret is shown to the user only at creation time; we cannot recover it later.
- The IPv6 reserved-range list is maintained by hand. There is a small risk of new reserved ranges being missed.
- HMAC does not protect against replay. A receiver that cares about freshness should use the `X-Promptsheon-Timestamp` header and reject events that are too old.

## References

- `internal/webhook/webhook.go` — dispatcher
- `internal/webhook/ssrf.go` — SSRF policy
- `docs/algorithms.md` — HMAC details
- `docs/security.md` — threat model
