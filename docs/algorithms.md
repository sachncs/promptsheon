# Algorithms

This page describes the algorithms that are central to Promptsheon but that live below the user-facing API. Each section has: a problem statement, a brief description of the approach, pseudocode, the source-of-truth Go file, and a pointer to the related ADR.

## Contents

- [BM25](#bm25)
- [Retry](#retry)
- [Circuit breaker](#circuit-breaker)
- [Fallback chain](#fallback-chain)
- [Cost calculation](#cost-calculation)
- [Vault (AES-256-GCM)](#vault-aes-256-gcm)
- [Audit chain](#audit-chain)
- [Webhook HMAC signing](#webhook-hmac-signing)
- [Workflow DAG execution](#workflow-dag-execution)
- [Token estimation and context truncation](#token-estimation-and-context-truncation)
- [Retention sweep](#retention-sweep)

---

## BM25

**Problem.** Given a corpus of prompt text, rank documents by relevance to a free-text query.

**Approach.** Standard Okapi BM25 with `k1=1.2`, `b=0.75`. Tokens are unigrams and bigrams with a light suffix strip for crude plural handling. Per-document length normalisation is applied.

**Source.** `internal/search/bm25.go`. ADR [0002](adr/0002-bm25-over-vector-search.md).

### Score

For each query term `q` and document `d`:

```
idf(q)  = ln(1 + (N - df(q) + 0.5) / (df(q) + 0.5))
tf(q,d) = number of times q appears in d
|d|     = length of d in tokens
avgdl   = average document length in the corpus
norm    = (1 - b) + b * (|d| / avgdl)

BM25(d,q) = idf(q) * (tf(q,d) * (k1 + 1)) / (tf(q,d) + k1 * norm)
```

The score for a query is the sum of the per-term scores. Documents with score `0` are not returned.

### Pseudocode

```text
function search(query, limit):
    tokens = tokenize(query)             # lowercase, split, suffix-strip, unigrams+bigrams
    if empty(tokens): return []
    N      = number of indexed documents
    avgdl  = total_tokens / N
    unique = set(tokens)                 # each term contributes once
    scores = []
    for d in docs:
        s = 0
        for q in unique:
            tf = term_frequency(d, q)
            if tf == 0: continue
            df = document_frequency(q)
            idf = ln(1 + (N - df + 0.5) / (df + 0.5))
            norm = (1 - b) + b * (doclen(d) / avgdl)
            s += idf * (tf * (k1 + 1)) / (tf + k1 * norm)
        if s > 0: scores.append((d, s))
    sort scores by score desc, docid asc
    return scores[:limit]
```

### Tokenisation

```text
function tokenize(text):
    text = lowercase(text)
    text = unicode-normalise NFKC
    tokens = []
    for word in split-on-non-letter(text):
        if length(word) < 2: continue
        word = strip-plural-suffix(word)   # trailing "s" -> "" if length > 3
        tokens.append(word)
        if length(word) >= 4:              # bigrams inside long words
            tokens.append(" "+word)
    return tokens
```

The index is in-process. The `Manager` wraps an `Index` and serialises `Add`/`Remove`/`Rebuild` behind a write lock; `Search` takes a read lock.

---

## Retry

**Problem.** LLM calls fail transiently. Naive retry storms the provider; not retrying turns brief blips into outages.

**Approach.** Exponential backoff with a typed-error classifier. Providers wrap retryable failures in `*llm.ErrTransient` and permanent ones in `*llm.ErrPermanent`. The classifier dispatches on those, plus a small set of well-known stdlib errors.

**Source.** `internal/llm/retry.go`. ADR [0007](adr/0007-slog-as-observability-foundation.md) (for the observability hooks).

### Classifier

```text
function is_retryable(err):
    if err is *ErrPermanent:             return false   # explicit
    if err is *ErrTransient:             return true    # explicit
    if err is context.Canceled:          return false
    if err is context.DeadlineExceeded:  return false
    if err is *CircuitOpen:              return false   # don't pile on
    if err is net.Error with Timeout():  return true
    if err is *net.OpError with Op=="dial": return true
    return true                                                 # permissive default
```

### Backoff

```text
delay(attempt) = min(base * 2^attempt, max)
```

The loop runs at most `MaxRetries + 1` total attempts. Cancellation via `ctx.Done()` is honoured during the backoff sleep.

### Defaults

```text
MaxRetries = 3
BaseDelay  = 500 ms
MaxDelay   = 10 s
```

---

## Circuit breaker

**Problem.** A failing provider should be shielded from further calls until it has had a chance to recover, but you also need to probe it periodically so you notice when it comes back.

**Approach.** Three-state machine: `closed` → `open` → `half-open` → `closed`. The state is held in process memory; it is not shared across server instances.

**Source.** `internal/llm/circuitbreaker.go`.

### State machine

```text
state: closed      failures=0, successes=0
state: open        since=lastFailureTime
state: half-open   successes=0

on Allow():
    if closed:   return true
    if open:     if now - since >= cooldown: state = half-open, return true
                 else:                        return false
    if half-open: return true

on RecordSuccess():
    if closed:   failures = 0
    if half-open: successes += 1
                 if successes >= successThreshold: state = closed, failures = 0, successes = 0

on RecordFailure():
    failures += 1
    since = now
    if closed:   if failures >= failureThreshold: state = open
    if half-open: state = open, successes = 0
```

### Defaults

```text
FailureThreshold = 5
SuccessThreshold = 3
Cooldown         = 30 s
```

When the breaker is open, calls return `ErrCircuitOpen` immediately. The retry classifier treats this as non-retryable, so the call is not retried.

### Environment overrides

```text
PROMPTSHEON_CIRCUIT_BREAKER_FAILURE_THRESHOLD
PROMPTSHEON_CIRCUIT_BREAKER_SUCCESS_THRESHOLD
PROMPTSHEON_CIRCUIT_BREAKER_COOLDOWN   # seconds
```

---

## Fallback chain

**Problem.** When the primary provider is down, requests should still complete (possibly with different latency or cost) rather than fail.

**Approach.** The dispatcher tries the primary first; on failure, it walks the fallback list in order. The first non-error response wins. Failures are logged at `warn` level; the final error is wrapped with the last provider's error.

**Source.** `internal/llm/fallback.go`.

```text
function complete(ctx, req):
    resp, err = primary.complete(ctx, req)
    if err == nil: return resp, nil
    log.warn("primary failed", "primary", primary.name, "err", err)
    for fb in fallbacks:
        if fb.name == primary.name: continue
        resp, err = fb.complete(ctx, req)
        if err == nil:
            log.info("fallback succeeded", "provider", fb.name)
            return resp, nil
        log.warn("fallback failed", "provider", fb.name, "err", err)
    return nil, fmt.Errorf("all providers failed, last error: %w", err)
```

### Environment

```text
PROMPTSHEON_LLM_FALLBACK=anthropic,ollama
```

The list is comma-separated. Whitespace is trimmed. Empty entries are ignored.

---

## Cost calculation

**Problem.** Estimate the USD cost of an LLM call from the model name and the token usage.

**Approach.** A static pricing table in `internal/llm/cost.go`. The table is keyed by the model string and stores `prompt_per_token` and `completion_per_token` in dollars.

**Source.** `internal/llm/cost.go`.

```text
function calculate_cost(model, usage):
    pricing = table[model]
    if not pricing: return 0
    return usage.prompt_tokens * pricing.prompt_per_token
         + usage.completion_tokens * pricing.completion_per_token
```

### Pricing table excerpt

| Model | Prompt $/1M | Completion $/1M |
|---|---|---|
| `gpt-4o` | 2.50 | 10.00 |
| `gpt-4o-mini` | 0.15 | 0.60 |
| `gpt-4-turbo` | 10.00 | 30.00 |
| `gpt-4` | 30.00 | 60.00 |
| `gpt-3.5-turbo` | 0.50 | 1.50 |
| `claude-sonnet-4-20250514` | 3.00 | 15.00 |
| `claude-3-5-sonnet-20241022` | 3.00 | 15.00 |
| `claude-3-5-haiku-20241022` | 0.80 | 4.00 |
| `claude-3-opus-20240229` | 15.00 | 75.00 |
| `llama3`, `mistral`, `codellama` | 0 | 0 |

Unknown models return `0` cost. The `metrics` package records the cost as a label on `llm_call_duration_seconds` and on the per-call `CallMetrics` struct.

---

## Vault (AES-256-GCM)

**Problem.** LLM provider API keys must be stored encrypted at rest. The encryption must be authenticated.

**Approach.** AES-256-GCM with a 32-byte key. A fresh 12-byte random nonce is generated for every encryption; the nonce is prepended to the ciphertext. The output is hex-encoded for storage.

**Source.** `internal/vault/vault.go`. ADR [0004](adr/0004-aes-256-gcm-vault.md).

### Key

The key is read from `PROMPTSHEON_VAULT_KEY` as a 64-character hex string. Three validation rules apply at startup:

1. Must be valid hex.
2. Must decode to exactly 32 bytes.
3. Must not be all zero (a misconfigured key with no entropy would otherwise produce ciphertexts that are trivially decryptable).

```text
function new(hex_key):
    key = hex.decode(hex_key)
    if len(key) != 32: error
    if all-zero(key):   error
    return Vault{key}
```

### Encrypt

```text
function encrypt(plaintext):
    block = aes.new(key)
    gcm   = gcm.new(block)
    nonce = random_bytes(12)
    ciphertext = gcm.seal(nonce, nonce, plaintext, nil)
    return hex.encode(ciphertext)
```

### Decrypt

```text
function decrypt(hex_ciphertext):
    ciphertext = hex.decode(hex_ciphertext)
    if len(ciphertext) < 12: error    # nonce + 0 bytes
    nonce, body = ciphertext[:12], ciphertext[12:]
    block = aes.new(key)
    gcm   = gcm.new(block)
    plaintext = gcm.open(nonce, body, nil)  # auth failure is an error
    return plaintext
```

GCM auth failure means the ciphertext was tampered with. We never return a partially-decrypted value in that case.

---

## Audit chain

**Problem.** Compliance reviewers need to verify, after the fact, that an audit row was not inserted, deleted, or modified.

**Approach.** Each `audit_entries` row carries the SHA-256 of a canonical representation of itself plus the `entry_hash` of its predecessor. A separate single-row `audit_chain_state` table caches the latest `entry_hash` so writes do not need to scan the full table.

**Source.** `internal/store/sqlite.go` (`AppendAudit`, `computeAuditHash`, `VerifyAuditChain`). Migrations `006`, `020`, `021`. ADR [0003](adr/0003-hash-chained-audit-log.md).

### Canonical form

The hash is computed over a `0x1f` (US, unit separator) delimited byte stream:

```text
entry_hash = sha256(
    id            || 0x1f ||
    user_id       || 0x1f ||
    action        || 0x1f ||
    resource      || 0x1f ||
    details_json  || 0x1f ||
    timestamp_str || 0x1f ||     # RFC3339Nano, UTC
    previous_hash
)
```

`details_json` is the JSON-encoded `Details` field. `timestamp_str` is the entry's timestamp serialised as RFC3339Nano in UTC. The binary `time.Time` is also stored, but the hash uses the canonical string for portability across timezones.

### Append

```text
function append_audit(entry):
    acquire in-process audit mutex
    begin serializable transaction
    prevHash = state.last_hash          # from audit_chain_state
    entry.previous_hash = prevHash
    entry.entry_hash    = compute_hash(entry)
    insert into audit_entries (...)
    upsert audit_chain_state set last_hash = entry.entry_hash, last_rowid = new_rowid
    commit
```

The mutex is in-process; the transaction is serializable. Together they prevent the read-then-write race that would otherwise fork the chain.

### Verify

```text
function verify_audit_chain():
    prev = ""
    after_rowid = 0
    loop:
        rows = select * from audit_entries where rowid > after_rowid order by rowid asc limit 1000
        if rows.empty: return ok
        for row in rows:
            if row.previous_hash != prev:           return fail("chain break at %s", row.id)
            if sha256(row) != row.entry_hash:       return fail("tampered entry %s", row.id)
            prev = row.entry_hash
            after_rowid = row.rowid
```

`GET /api/v1/audit/verify` returns `{"ok": true}` on success or `{"ok": false, "reason": "..."}` on the first failure. The verifier pages in chunks of 1000 rows so a long chain does not hold a single connection.

---

## Webhook HMAC signing

**Problem.** Receivers must be able to tell, for any given delivery, whether it came from Promptsheon and was not tampered with in transit.

**Approach.** Every webhook delivery carries an `X-Promptsheon-Signature: sha256=<hex>` header. The hex value is `HMAC-SHA256(secret, raw_body)`. The secret is per-endpoint, generated at registration time, and returned to the user once.

**Source.** `internal/webhook/webhook.go`. ADR [0005](adr/0005-hmac-webhooks-with-ssrf-allowlist.md).

```text
function sign(secret, body):
    mac = hmac.new(secret, body, sha256)
    return "sha256=" + mac.hexdigest()

function deliver(event, endpoint):
    body = json.encode(event)
    headers = {
        "Content-Type":          "application/json",
        "X-Promptsheon-Event":   event.type,
        "X-Promptsheon-Delivery": event.id,
        "X-Promptsheon-Signature": sign(endpoint.secret, body),
    }
    response = http.post(endpoint.url, body, headers)
    record delivery outcome
```

### Verifier (in receiver)

```text
expected = "sha256=" + hmac_sha256(secret, body)
if not constant_time_equal(expected, header_signature):
    reject 401
```

Always use a constant-time comparison. Treat any other header value as a failure.

---

## Workflow DAG execution

**Problem.** Execute a collection of steps with explicit dependencies in the right order, with bounded concurrency and predictable failure handling.

**Approach.** Validate the graph, compute a topological level for each step, then execute level by level. Cycles are rejected.

**Source.** `internal/workflow/`. ADR [0008](adr/0008-workflow-dag-with-topological-execution.md).

### Validate

```text
function validate(steps):
    graph = adjacency_from(steps, "depends_on")
    for step in steps:
        if dfs_has_cycle(graph, step): return error("cycle detected")
```

### Topological sort (Kahn's algorithm)

```text
function topological_levels(steps):
    in_degree = count(steps, "depends_on")
    levels    = []
    ready     = {s for s in steps if in_degree[s] == 0}
    while ready not empty:
        levels.append(ready)
        next_ready = {}
        for s in ready:
            for child in steps where s in child.depends_on:
                in_degree[child] -= 1
                if in_degree[child] == 0: next_ready.add(child)
        ready = next_ready
    if any(in_degree > 0): error("cycle detected")
    return levels
```

### Execute

```text
function execute(levels, ctx):
    results = {}
    for level in levels:
        goroutines = []
        for step in level:
            goroutines.append(run(step, ctx, results))   # inputs come from results
        wait(goroutines)
        for step in level:
            if step.failed: mark_descendants(step, status="skipped")
```

The runtime status per step is one of: `pending`, `running`, `completed`, `failed`, `skipped`, `cancelled`. The state is persisted after every transition.

### Cancellation

```text
on ctx.Done():
    for in_flight step:
        cancel step's goroutine
        mark step status = "cancelled"
    do not start remaining levels
```

Cancellation is cooperative. Steps that are blocked on the LLM call get a `ctx.Err()` from the per-step timeout, which the retry classifier treats as non-retryable.

---

## Token estimation and context truncation

**Problem.** A conversation history can grow past the LLM's context window. We need to estimate token count without a real tokenizer and to apply a truncation strategy when the budget is exceeded.

**Approach.** Word-count heuristic at ~1.3 tokens per word. Three truncation strategies: `none`, `sliding_window`, `summarize`. The strategy is configured per context.

**Source.** `internal/context/manager.go`.

```text
function default_token_estimate(text):
    if text == "": return 0
    words = split_on_whitespace(text)
    return floor(len(words) * 1.3)

function assemble(context, variables, budget):
    system = render_template(context.system_prompt, variables)
    tokens = estimate(system)
    messages = clone(context.messages)               # most-recent first
    truncated = false
    for msg in messages:
        if tokens + estimate(msg) > budget:
            truncated = true
            switch context.strategy:
                case none:           drop remaining
                case sliding_window:  drop remaining
                case summarize:      replace dropped block with summary placeholder
            break
        tokens += estimate(msg)
    return {system, messages, tokens, truncated, strategy}
```

The estimator is intentionally rough. The point is to not blow past the window; we do not need exact token counts. For exact counts, the `llm` package exposes `Tokenizer` (see `internal/llm/tokenizer.go`).

---

## Retention sweep

**Problem.** Trace spans, audit entries, and snapshots grow without bound. We need a periodic cleanup that respects per-table TTLs.

**Approach.** A background goroutine that wakes every `CheckInterval` and deletes rows older than the configured TTL. The minimum trace retention is 30 days (regulatory floor) and is enforced even if a smaller value is configured.

**Source.** `internal/observability/retention.go`.

```text
function load_retention_policy_from_env():
    policy = defaults()
    if env("PROMPTSHEON_TRACE_TTL_DAYS"):    policy.trace_ttl    = days * 24h
    if env("PROMPTSHEON_SNAPSHOT_TTL_DAYS"): policy.snapshot_ttl = days * 24h
    if env("PROMPTSHEON_AUDIT_TTL_DAYS"):    policy.audit_ttl    = days * 24h
    if env("PROMPTSHEON_RETENTION_CHECK_MINUTES"): policy.check_interval = minutes * 1m
    if policy.trace_ttl < 30 days: policy.trace_ttl = 30 days
    return policy

function sweep():
    now = time.now()
    delete from traces    where started_at < now - policy.trace_ttl
    delete from snapshots where created_at < now - policy.snapshot_ttl
    delete from audit_entries where timestamp < now - policy.audit_ttl
```

### Defaults

```text
TraceTTL      = 30 days
SnapshotTTL   = 30 days
AuditTTL      = 90 days
CheckInterval = 1 hour
```

Audit entries older than `AuditTTL` are deleted, but the hash chain is still verifiable over the surviving window. If you need a longer audit retention, raise the TTL; the sweeper respects the configured value.
