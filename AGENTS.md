# AGENTS.md

# TypeScript Engineering Constitution

Version: 2.0

---

# Mission

This repository is expected to represent production-grade TypeScript engineering.

The actual frontend stack is **Next.js 16 (App Router) + TanStack Query + axios + shadcn/ui** running on top of a **Fastify + better-sqlite3** backend. The repo is a pnpm workspace: `packages/{shared,server,cli,sdk}/` plus `frontend/`. **Not Vite, not React Router v7** — those references in earlier docs are stale.

Every change must improve the repository.

Every contribution must increase one or more of the following qualities:

- Correctness
- Readability
- Simplicity
- Maintainability
- Modularity
- Extensibility
- Reliability
- Resilience
- Security
- Scalability
- Performance
- Testability
- Consistency

Working code alone is **not** considered complete.

---

# Primary Engineering References

The following references define the engineering standards for this repository.

Follow them in order of precedence.

1. TypeScript Handbook
2. Google TypeScript Style Guide
3. Effective TypeScript
4. TypeScript Deep Dive
5. Standard Library Documentation
6. Repository-specific rules defined in this document

If repository rules are stricter than the language conventions, the repository rules take precedence.

---

# Engineering Philosophy

Every engineering decision should optimise for long-term software quality.

Always prioritise

- correctness over cleverness
- simplicity over complexity
- readability over brevity
- maintainability over optimisation
- architecture over implementation
- explicit behaviour over implicit behaviour
- deterministic behaviour over convenience

Avoid introducing complexity unless it solves a measurable problem.

Software should become easier to understand after every contribution.

---

# Agent Workflow

Never immediately modify source files.

Every task shall follow the workflow below.

## Phase 1 — Repository Analysis

Before writing code, inspect the repository. Understand the structure, package organisation, architecture, existing abstractions, naming conventions, coding patterns, dependencies, tests, and tooling.

## Phase 2 — Problem Understanding

Identify root cause, affected packages, downstream dependencies, architectural implications, API compatibility, concurrency implications, security implications, performance implications.

Never fix symptoms. Always fix the underlying cause.

## Phase 3 — Planning

Minimise complexity, minimise coupling, maximise cohesion, reuse existing abstractions, preserve package boundaries, maintain backwards compatibility.

Do not introduce unnecessary packages or abstractions.

## Phase 4 — Design Review

Before adding new code, ask: can this be solved by extending an existing package, improving an existing abstraction, simplifying existing logic, removing duplication, or reusing an existing component?

Prefer improving existing code over creating new code.

## Phase 5 — Implementation

Implement changes incrementally. The repository should remain in a working state after each logical modification.

## Phase 6 — Self Review

Inspect naming, package design, interface design, readability, simplicity, error handling, concurrency, documentation, tests, performance, security.

Assume the code will be maintained by someone unfamiliar with the project.

## Phase 7 — Refactoring

Do not stop after making the code work. Improve organisation, naming, interfaces, documentation, tests, readability, duplication, architectural consistency.

Leave the repository healthier than before.

## Phase 8 — Verification

Verify builds succeed, tests pass, type checks pass, formatting passes, examples remain valid, documentation is synchronised, exported APIs remain consistent.

## Phase 9 — Completion

A task is complete only after every engineering requirement in this document has been satisfied.

---

# Engineering Principles

## KISS

Keep It Simple. Choose the simplest design that completely satisfies the requirements. Avoid unnecessary abstraction, configuration, indirection, optimisation, and layers.

## DRY

Don't Repeat Yourself. Every piece of knowledge should exist in one authoritative location. Avoid duplicated algorithms, validation, configuration, constants, business rules, serialization, and parsing logic.

## YAGNI

You Aren't Gonna Need It. Do not implement speculative APIs, interfaces, configuration, extension points, or optimisations. Implement only what is currently required.

## SOLID

Apply SOLID where appropriate within TypeScript's composition-oriented design.

- Single Responsibility: each module has one reason to change
- Open/Closed: extend behaviour without modifying existing code
- Liskov Substitution: subtypes are substitutable for their base types
- Interface Segregation: prefer small, focused interfaces
- Dependency Inversion: depend on abstractions, not implementations

## Composition Over Inheritance

TypeScript favours composition and interfaces over inheritance. Use composition, embedding, and interfaces rather than emulating inheritance.

## Separation of Concerns

Each package should have one primary responsibility. Separate domain logic, persistence, networking, configuration, validation, serialization, and presentation.

## Single Source of Truth

Each concept should have exactly one authoritative implementation.

## Principle of Least Astonishment

Code should behave exactly as experienced TypeScript developers expect. Avoid hidden behaviour, surprising APIs, unexpected side effects, implicit state.

## Fail Fast

Detect invalid state immediately. Reject invalid input early. Never silently ignore failures.

## Defensive Programming

Assume invalid input, corrupted files, malformed requests, unavailable dependencies, partial failures. Software should fail predictably and recover gracefully.

## High Cohesion

Packages, files, types, and functions should represent one concept.

## Low Coupling

Reduce dependencies between packages. Depend on behaviour rather than implementation.

## Determinism

Identical inputs should produce identical outputs unless explicitly documented otherwise.

---

# TypeScript Language Standards

## General Philosophy

Write idiomatic TypeScript. Do not write Java in TypeScript. Do not write C++ in TypeScript. Do not write Python in TypeScript.

Prefer simple, explicit, readable code.

## Repository Consistency

Every change must preserve consistency across the repository. Before creating packages, interfaces, structs, helper functions, or utilities, inspect the repository. If an appropriate abstraction already exists, reuse it.

## Package Design

Packages are the primary architectural boundary. A package should represent one cohesive concept. Avoid "miscellaneous" packages.

## Package Naming

Package names must be lowercase, contain no underscores, contain no hyphens, be concise, and describe the domain.

## Variables

Variable names should be concise and descriptive. Small scope allows `err`, `ctx`, `buf`, `req`. Long-lived variables require descriptive names.

## Functions

Functions should perform one responsibility, remain deterministic, remain independently testable, and remain readable.

## Types

Types should model domain concepts. Avoid anemic data models. A type should preserve its own invariants.

## Constructors

Use constructors only when initialization is required. The zero value should be useful whenever practical.

## Interfaces

Interfaces describe behaviour. Keep interfaces as small as possible. Most interfaces should contain one to three methods.

## Composition

Prefer composition. Prefer embedding. Avoid emulating inheritance.

## Generics

Use generics only when they reduce duplication. Do not introduce generic abstractions prematurely.

## Constants

Avoid magic values. Replace with named constants.

## Enumerations

Prefer typed constants. Use string literal unions rather than `enum` (TypeScript style guide recommendation).

## Imports

Imports should be minimal, organised, and automatically formatted. Never leave unused imports. Never create import cycles.

## Error Handling

Every returned error must be handled, returned, or wrapped. Wrap errors with context using `Error` or a custom subclass.

## Panic

Do not throw for expected failures. Recoverable failures should return errors.

## Async / Await

Always use `async`/`await` over raw Promise chains. Use `Promise.all` for parallel operations. Handle rejection.

## Context

Use `AbortController` for request-scoped cancellation. Honour cancellation in long-running operations.

## Global State

Avoid mutable global state. Avoid hidden initialisation. Prefer explicit dependency injection.

## init()

Avoid module-level `init()` patterns. Initialisation should be explicit.

## Logging

Use structured logging. Never log passwords, API keys, secrets, credentials, or tokens.

## Configuration

Never hardcode credentials, ports, URLs, file paths, or secrets. Configuration should be externalised.

## Reflection

Avoid reflection unless necessary. Reflection reduces readability and type safety.

## unsafe

Avoid `any` and unsafe type assertions. Use `unknown` and proper type guards.

## Comments

Comments explain **why**, not **what**. Delete commented-out code.

## Documentation

Every exported identifier requires TSDoc. Documentation should explain purpose, behaviour, guarantees, constraints.

## Public APIs

Public APIs should remain stable, documented, minimal, and backwards compatible. Breaking changes require documentation and migration guidance.

---

# Package Architecture

## Goals

Every architectural decision should reduce coupling, increase cohesion, improve readability, simplify maintenance, improve testing, encourage reuse, isolate responsibilities, and preserve backwards compatibility.

## Package First Design

Packages are the primary architectural unit. Design packages before designing types.

## Internal Packages

Use `internal/` for implementation details that must not become public APIs.

## Public Packages

Public packages should expose only stable APIs, minimal APIs, and well-documented APIs.

## Domain Modeling

Model domain concepts explicitly. Domain terminology improves readability.

## Dependency Injection

Dependencies should be injected. Avoid constructing dependencies internally.

## Configuration

Configuration belongs in dedicated configuration types. Configuration should be immutable after initialization.

## State Management

Avoid mutable shared state. Avoid global caches. Avoid singleton objects. State should have one owner.

## Lifecycle Management

Every long-lived component should have a clearly defined lifecycle. Resources must always be released.

---

# Code Quality Checklist

Before considering implementation complete verify:

- Code follows the Google TypeScript Style Guide
- Code follows Effective TypeScript
- Package design is cohesive
- Interfaces remain small
- Composition is preferred over inheritance-like designs
- Constructors validate invariants
- Zero-value usability is preserved
- Errors are handled correctly
- Errors are wrapped with context
- Resources are cleaned up
- Imports remain clean
- Exported APIs have TSDoc
- Public APIs remain stable
- No duplicate logic exists
- No dead code exists
- No placeholder implementations remain
- Repository consistency has been preserved

Code is considered complete only when it satisfies every requirement above.

---

# Repository-Specific Rules

## TypeScript Configuration

- `strict: true` is required
- `noImplicitAny: true` is required
- `strictNullChecks: true` is required
- `noUncheckedIndexedAccess: true` is required
- `exactOptionalPropertyTypes: true` is required

## Validation

- Use `zod` for runtime validation of all external inputs
- Define schemas in `packages/shared/src/validation.ts`
- Never use `as Type` casts to bypass validation

## Database

- Use `better-sqlite3` prepared statements
- All queries go through repos in `packages/server/src/repos/`
- No raw SQL in route handlers

## AI / LLM

- All LLM operations use `@strands-agents/sdk`
- All agents live in `packages/server/src/agents/`
- Use `AgentResult.lastMessage.content` (not `.message`)
- Extract text via `extractText()` helper

## HTTP

- All routes use Fastify in `packages/server/src/routes/`
- All request bodies validated via Zod
- All errors returned as `{ error: { code, message } }`

## Frontend

- React 19 + Vite + shadcn/ui
- TanStack Query for all data fetching
- React Router v7 (HashRouter)
- 15 UI components, 11 modals, 25 views
