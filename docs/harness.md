# Harness

In the [OpenAI harness engineering article](https://openai.com/index/harness-engineering/),
the word "harness" refers to the scaffolding around an LLM — the
evals, the precondition hooks, the fast feedback loops — that
turns an LLM into a tool you can ship with confidence.

In Promptsheon, the harness is the
[Capabilities](architecture.md#capabilities) →
[Versions](architecture.md#versions) →
[Releases](release.md) → **Evals** pipeline. The Release lifecycle
is gated on a set of named command hooks (preconditions), and the
Release can be evaluated against a Dataset of test cases using a
registered scorer. The score is the green/red signal that the
release engineer's iteration loop is wired to.

## The four surfaces

1. **Capability** — the business outcome the user wants.
2. **Version** — an immutable build of the Manifest that addresses
   that Capability.
3. **Release** — a Version approved for a target Environment.
   Activation runs the Capability's preconditions; a failing hook
   blocks the Release.
4. **Eval** — a recorded scoring run of a Release against a
   Dataset. Score is `passed / total`. The Run becomes the
   per-Version progress signal in the iteration loop.

## The invariant

Exactly one **Active Release per (Capability, Environment)** at
any time. Activating a new Release supersedes the prior Active
Release atomically (one transaction). Preconditions run before
the supersede + activate write commits; an Activate that fails
the precondition gate leaves the prior Active Release untouched
and returns 409.

## What the harness doesn't do

- It does not write code or open PRs. The harness is the
  evaluation and gating layer, not the agent itself.
- It does not run the LLM. The daemon routes eval invocations
  through the same `invoke.Invoker` used by the live
  `/releases/{id}/invoke` route, so eval cases use the same
  provider wiring.
- It does not store agent outputs. Per-case EvalResults persist
  the raw `actual` value plus pass/fail + latency.

## Where to read next

- [eval.md](eval.md) — the eval primitive in detail.
- [release.md](release.md) — the Release lifecycle and the
  MakerChecker approval gate.
- [architecture.md](architecture.md) — where harness fits in the
  larger Capability/Version/Release/Eval stack.
- The OpenAI article that inspired the surface:
  [Harness engineering: leveraging Codex in an agent-first world](https://openai.com/index/harness-engineering/).
