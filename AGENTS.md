# AGENTS.md

# Go Engineering Constitution

Version: 1.0

---

# Mission

This repository is expected to represent production-grade Go engineering.

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

The objective is to build software that remains understandable, maintainable, and reliable years after its initial implementation.

---

# Primary Engineering References

The following references define the engineering standards for this repository.

Follow them in order of precedence.

1. Go Language Specification
2. Google Go Style Guide
   https://google.github.io/styleguide/go/
3. Effective Go
4. Go Code Review Comments
5. Go Proverbs
6. Standard Library Documentation
7. Repository-specific rules defined in this document

If repository rules are stricter than the language conventions, the repository rules take precedence.

Never invent style rules when an official guideline already exists.

---

# Engineering Philosophy

Every engineering decision should optimize for long-term software quality.

Always prioritize

- correctness over cleverness
- simplicity over complexity
- readability over brevity
- maintainability over optimization
- architecture over implementation
- explicit behavior over implicit behavior
- deterministic behavior over convenience

Avoid introducing complexity unless it solves a measurable problem.

Software should become easier to understand after every contribution.

---

# Agent Responsibilities

Every AI coding agent is expected to behave as a

- senior software engineer
- software architect
- code reviewer
- maintainer
- tester
- documentation author

The objective is **not** to merely satisfy the immediate request.

The objective is to improve the repository while preserving correctness, consistency, and maintainability.

Every task should reduce technical debt whenever practical.

---

# Agent Workflow

Never immediately modify source files.

Every task shall follow the workflow below.

---

## Phase 1 — Repository Analysis

Before writing code, inspect the repository.

Understand

- repository structure
- package organization
- architecture
- existing abstractions
- interfaces
- exported APIs
- internal packages
- naming conventions
- coding patterns
- dependencies
- tests
- documentation
- tooling
- build process

Do not assume.

Infer conventions from the repository.

---

## Phase 2 — Problem Understanding

Understand the problem completely.

Identify

- root cause
- affected packages
- downstream dependencies
- architectural implications
- API compatibility
- concurrency implications
- security implications
- performance implications

Never fix symptoms.

Always fix the underlying cause.

---

## Phase 3 — Planning

Before implementation, create a plan.

The implementation should

- minimize complexity
- minimize coupling
- maximize cohesion
- reuse existing abstractions
- preserve package boundaries
- maintain backwards compatibility whenever practical

Do not introduce unnecessary packages or abstractions.

---

## Phase 4 — Design Review

Before adding new code, ask

Can this be solved by

- extending an existing package?
- extending an existing interface?
- improving an existing abstraction?
- simplifying existing logic?
- removing duplication?
- reusing an existing component?

Prefer improving existing code over creating new code.

---

## Phase 5 — Implementation

Implement changes incrementally.

The repository should remain in a working state after each logical modification.

Large changes should be broken into smaller, reviewable steps.

---

## Phase 6 — Self Review

Review every change before considering it complete.

Inspect

- naming
- package design
- interface design
- readability
- simplicity
- error handling
- concurrency
- documentation
- tests
- performance
- security

Assume the code will be maintained by someone unfamiliar with the project.

---

## Phase 7 — Refactoring

Do not stop after making the code work.

Improve

- package organization
- naming
- interfaces
- documentation
- tests
- readability
- duplication
- architectural consistency

Leave the repository healthier than before.

---

## Phase 8 — Verification

Verify

- builds succeed
- tests pass
- race detector passes
- static analysis passes
- formatting passes
- examples remain valid
- documentation is synchronized
- exported APIs remain consistent

---

## Phase 9 — Completion

A task is complete only after every engineering requirement in this document has been satisfied.

---

# Engineering Principles

Every design decision should be evaluated against these principles.

Violations require explicit technical justification.

---

## KISS

Keep It Simple.

Choose the simplest design that completely satisfies the requirements.

Avoid

- unnecessary abstraction
- unnecessary configuration
- unnecessary indirection
- unnecessary optimization
- unnecessary layers

Complexity requires justification.

---

## DRY

Don't Repeat Yourself.

Every piece of knowledge should exist in one authoritative location.

Avoid duplicated

- algorithms
- validation
- configuration
- constants
- business rules
- serialization logic
- parsing logic

Extract reusable abstractions where appropriate.

---

## YAGNI

You Aren't Gonna Need It.

Do not implement

- speculative APIs
- speculative interfaces
- speculative configuration
- speculative extension points
- speculative optimizations

Implement only what is currently required.

---

## SOLID

Apply SOLID where appropriate within Go's composition-oriented design.

Every implementation should respect

- Single Responsibility Principle
- Open/Closed Principle
- Liskov Substitution Principle
- Interface Segregation Principle
- Dependency Inversion Principle

Go favors composition and interfaces over inheritance.

Apply SOLID idiomatically rather than mechanically.

---

## Composition Over Inheritance

Go does not support class inheritance.

Favor

- composition
- embedding
- interfaces

Avoid designs that attempt to emulate inheritance.

Compose behavior instead.

---

## Separation of Concerns

Each package should have one primary responsibility.

Separate

- domain logic
- persistence
- networking
- configuration
- validation
- serialization
- presentation

Avoid mixing unrelated concerns.

---

## Single Source of Truth

Each concept should have exactly one authoritative implementation.

Avoid duplicated

- validation
- schemas
- configuration
- constants
- business logic

---

## Principle of Least Astonishment

Code should behave exactly as experienced Go developers expect.

Avoid

- hidden behavior
- surprising APIs
- unexpected side effects
- implicit state

Favor explicitness.

---

## Law of Demeter

Packages should expose minimal APIs.

Objects should communicate only with their direct collaborators.

Avoid deep call chains.

Prefer concise package interfaces.

---

## Fail Fast

Detect invalid state immediately.

Reject invalid input early.

Do not continue execution after detecting corrupted state.

Never silently ignore failures.

---

## Design by Contract

Public APIs should clearly define

- expected inputs
- valid outputs
- error conditions
- invariants

Validate assumptions.

Return informative errors.

---

## Defensive Programming

Assume

- invalid input
- corrupted files
- malformed requests
- unavailable dependencies
- partial failures
- network failures
- resource exhaustion

Software should fail predictably and recover gracefully whenever appropriate.

---

## High Cohesion

Packages should contain closely related functionality.

Files should represent one concept.

Types should have one responsibility.

Functions should do one thing well.

---

## Low Coupling

Reduce dependencies between packages.

Depend on behavior rather than implementation.

Avoid unnecessary package imports.

Avoid cyclic dependencies.

---

## Determinism

Identical inputs should produce identical outputs unless explicitly documented otherwise.

Avoid hidden state.

Avoid implicit configuration.

Avoid unpredictable behavior.

---

## Repository Evolution

Every contribution should improve

- architecture
- readability
- maintainability
- documentation
- testing
- consistency
- simplicity

The repository should become easier to maintain after every change.

---

# Agent Persistence

Do not stop after producing the first working implementation.

Continue improving the implementation until

- requested functionality is complete
- documentation is updated
- tests have been added or improved
- architectural consistency has been restored
- duplication has been removed
- dead code has been eliminated
- naming has been improved
- package boundaries remain coherent
- exported APIs remain consistent
- all quality gates have been satisfied

Never knowingly leave behind

- TODO comments
- FIXME comments
- placeholder implementations
- stub functions
- suppressed diagnostics
- ignored errors
- dead code
- duplicate logic
- incomplete refactoring
- inconsistent behavior

The implementation is complete only when it is

- production ready
- idiomatic Go
- well documented
- comprehensively tested
- architecturally sound
- fully compliant with every requirement in this document.

---

# Go Language Standards

All Go code shall follow, in order of precedence

1. Go Language Specification
2. Google Go Style Guide
3. Effective Go
4. Go Code Review Comments
5. Standard Library Documentation
6. Repository-specific rules defined in this document

When this repository defines stricter requirements, the repository rules take precedence.

---

# General Philosophy

Write idiomatic Go.

Do not write Java in Go.

Do not write C++ in Go.

Do not write Python in Go.

Prefer simple, explicit, readable code.

Readable code is always preferred over clever code.

---

# Repository Consistency

Every change must preserve consistency across the repository.

Before creating

- packages
- interfaces
- structs
- helper functions
- utilities
- abstractions

inspect the repository.

If an appropriate abstraction already exists

reuse it.

Do not duplicate functionality.

---

# Package Design

Packages are the primary architectural boundary.

A package should represent one cohesive concept.

Packages should be

- cohesive
- focused
- reusable
- independently understandable

Avoid "miscellaneous" packages.

Avoid dumping unrelated functionality into the same package.

---

# Package Naming

Package names must

- be lowercase
- contain no underscores
- contain no hyphens
- be concise
- describe the domain

Good

```
storage

repository

parser

config

validator
```

Bad

```
helpers

common

misc

utilities

stuff
```

---

# Package Responsibilities

Every package should have one responsibility.

Bad

```
repository/

contains

database

networking

validation

serialization
```

Good

Separate packages.

---

# File Organization

Files should remain focused.

Prefer grouping by responsibility rather than size.

Avoid enormous files.

If a file becomes difficult to navigate

split it.

---

# Naming

Naming is one of the most important engineering decisions.

Names should immediately communicate intent.

Use complete words.

Prefer domain terminology.

Avoid abbreviations unless universally understood.

---

# Forbidden Names

Avoid

```
tmp

temp

foo

bar

baz

obj

mgr

util

helper

misc

thing

stuff

manager
```

Names should describe responsibility rather than implementation.

---

# Variables

Variable names should be concise and descriptive.

Small scope

```
err

ctx

buf

req
```

are acceptable.

Long-lived variables require descriptive names.

Good

```
repository

configuration

connectionPool

retryCount
```

Bad

```
x

data

value

obj
```

---

# Functions

Functions should

- perform one responsibility
- remain deterministic
- remain independently testable
- remain readable

Avoid long functions.

Extract reusable logic.

---

# Function Design

Prefer

```
LoadConfiguration()
```

over

```
DoConfigurationStuff()
```

Function names should describe behavior.

---

# Function Parameters

Avoid long parameter lists.

When multiple parameters represent one concept

create a struct.

Prefer

```
DatabaseConfiguration
```

instead of

```
host

port

username

password

database
```

---

# Function Size

Small functions improve

- readability
- reuse
- testing
- maintainability

Large functions usually indicate missing abstractions.

---

# Structs

Structs should model domain concepts.

Avoid anemic data models.

Avoid structs with unrelated responsibilities.

A struct should preserve its own invariants.

---

# Constructors

Use constructors only when initialization is required.

Constructors should

- validate invariants
- initialize required dependencies
- return valid objects

Prefer

```
NewRepository()
```

when initialization matters.

Avoid unnecessary constructors.

The zero value should be useful whenever practical.

---

# Zero Value

Types should remain usable with their zero value whenever possible.

Avoid requiring initialization unless necessary.

Design APIs that behave correctly by default.

---

# Interfaces

Interfaces describe behavior.

They do not describe implementation.

Prefer defining interfaces in the consuming package.

Keep interfaces as small as possible.

Most interfaces should contain one to three methods.

---

# Accept Interfaces

Functions should accept interfaces when appropriate.

Example

```
func Save(storage Storage)
```

instead of

```
func Save(database PostgreSQLDatabase)
```

---

# Return Concrete Types

Constructors should generally return concrete types.

Avoid returning interfaces unless there is a compelling architectural reason.

---

# Composition

Prefer composition.

Prefer embedding.

Avoid designs that simulate inheritance.

Behavior should emerge through composition.

---

# Embedding

Embedding should simplify APIs.

Do not use embedding merely to expose unrelated methods.

Avoid confusing method promotion.

---

# Generics

Use generics only when they reduce duplication.

Do not introduce generic abstractions prematurely.

Prefer simple concrete implementations.

---

# Constants

Avoid magic values.

Replace

```
86400
```

with

```
SecondsPerDay
```

Constants should describe meaning.

---

# Enumerations

Prefer typed constants.

Use

```
iota
```

only when it improves readability.

Avoid exposing meaningless numeric values.

---

# Imports

Imports should be

- minimal
- organized
- automatically formatted

Never leave unused imports.

Never create import cycles.

Prefer standard library packages whenever possible.

---

# Error Handling

Every returned error must be

- handled
- returned
- wrapped

Never ignore errors.

Forbidden

```
_, _ = file.Write(data)

value, _ := parse()

_ = close()
```

Every ignored error requires documented justification.

---

# Error Wrapping

Wrap errors with context.

Prefer

```
fmt.Errorf("load configuration: %w", err)
```

rather than replacing the original error.

Preserve the error chain.

---

# Panic

Do not panic for expected failures.

Panic only when

- programmer errors
- impossible states
- unrecoverable corruption

Recoverable failures should return errors.

---

# Deferred Cleanup

Always clean resources using

```
defer
```

Examples

- files

- mutexes

- transactions

- network connections

- HTTP responses

---

# Context

Use

```
context.Context
```

for request-scoped operations.

Context should always be the first parameter.

Good

```
func Execute(ctx context.Context, ...)
```

Bad

```
func Execute(..., ctx context.Context)
```

---

# Context Rules

Never store

```
context.Context
```

inside a struct.

Never pass nil contexts.

Use

```
context.Background()

context.TODO()
```

when appropriate.

Always propagate context.

Honor cancellation.

---

# Global State

Avoid mutable global state.

Avoid hidden initialization.

Prefer explicit dependency injection.

---

# init()

Avoid

```
init()
```

unless absolutely necessary.

Initialization should be explicit.

---

# Logging

Use structured logging.

Follow the project logging standard.

Never use logging for flow control.

Never log

- passwords
- API keys
- secrets
- credentials
- tokens

---

# Configuration

Never hardcode

- credentials
- ports
- URLs
- file paths
- secrets

Configuration should be externalized.

---

# Reflection

Avoid reflection unless there is no simpler solution.

Reflection reduces readability and type safety.

---

# Unsafe

Avoid

```
unsafe
```

unless absolutely necessary.

Every use requires explicit documentation.

---

# Comments

Comments explain

why

not

what

Delete commented-out code.

---

# Documentation

Every exported identifier requires GoDoc.

Documentation should explain

- purpose
- behavior
- guarantees
- constraints

Keep documentation synchronized with implementation.

---

# Public APIs

Public APIs should remain

- stable
- documented
- minimal
- backwards compatible

Breaking changes require

- documentation
- migration guidance
- updated examples
- updated tests

---

# Repository Consistency

Whenever a public API changes

update every affected location

including

- tests
- examples
- README
- documentation
- benchmarks
- tutorials
- CHANGELOG

The repository should remain internally consistent at all times.

---

# Go Quality Checklist

Before considering implementation complete verify

✓ Code follows the Google Go Style Guide.

✓ Code follows Effective Go.

✓ Package design is cohesive.

✓ Interfaces remain small.

✓ Composition is preferred over inheritance-like designs.

✓ Constructors validate invariants.

✓ Zero-value usability is preserved.

✓ Errors are handled correctly.

✓ Errors are wrapped with context.

✓ Context is propagated correctly.

✓ Resources are cleaned up.

✓ Imports remain clean.

✓ Exported APIs have GoDoc.

✓ Public APIs remain stable.

✓ No duplicate logic exists.

✓ No dead code exists.

✓ No placeholder implementations remain.

✓ Repository consistency has been preserved.

Code is considered complete only when it satisfies every requirement above.

---

# Package Architecture and Software Design

The architecture of this repository shall prioritize

- simplicity
- modularity
- maintainability
- extensibility
- reliability
- scalability
- readability
- testability

Architecture exists to reduce complexity.

Do not introduce architectural patterns unless they solve a measurable problem.

---

# Architectural Goals

Every architectural decision should

- reduce coupling
- increase cohesion
- improve readability
- simplify maintenance
- improve testing
- encourage reuse
- isolate responsibilities
- preserve backwards compatibility

The architecture should become simpler over time.

---

# Package First Design

Packages are the primary architectural unit in Go.

Design packages before designing structs.

Every package should represent one business capability or one technical capability.

A package should answer one question:

"What responsibility does this package own?"

If multiple unrelated answers exist, split the package.

---

# Package Responsibilities

Each package must have one primary responsibility.

Examples

Good

```
config

repository

storage

parser

validator

cache

authentication
```

Poor

```
common

helpers

shared

utilities

misc

general
```

Package names must describe domain concepts rather than implementation details.

---

# Package Boundaries

Package boundaries should be explicit.

Business logic should never leak into unrelated packages.

Avoid

- cyclic dependencies
- hidden dependencies
- tightly coupled packages
- implicit package contracts

---

# Package Dependencies

Dependencies should always point inward toward stable abstractions.

High-level packages should not depend directly on implementation details.

Prefer

```
API

↓

Service

↓

Repository Interface

↓

Storage Implementation
```

Avoid

```
API

↓

Database

↓

Network

↓

Configuration

↓

Utility

↓

Everything
```

---

# Internal Packages

Use

```
internal/
```

for implementation details that must not become public APIs.

Do not expose internal implementation accidentally.

Anything inside

```
internal/
```

may evolve without affecting external consumers.

---

# Public Packages

Public packages should expose only

- stable APIs
- minimal APIs
- well documented APIs

Avoid exporting implementation details.

Export behavior rather than state.

---

# Package Layout

Prefer a logical package structure.

Example

```
cmd/

internal/

api/

application/

domain/

repository/

storage/

configuration/

authentication/

validation/

types/

errors/

tests/
```

Do not introduce unnecessary directory depth.

Directory hierarchy should reflect architectural boundaries.

---

# Domain Modeling

Model domain concepts explicitly.

Prefer

```
Repository

Document

User

Invoice

Configuration
```

Avoid

```
Manager

Helper

Utility

Engine

Processor
```

Domain terminology improves readability.

---

# Abstraction

Expose only meaningful behavior.

Hide implementation details.

Consumers should understand

what

without needing to understand

how.

Avoid leaking implementation details across package boundaries.

---

# Encapsulation

Protect internal state.

Expose minimal APIs.

Avoid exporting mutable fields.

Prefer methods over direct state manipulation.

Example

Prefer

```
repository.Add(document)
```

Instead of

```
repository.Documents = append(...)
```

Objects should preserve their own invariants.

---

# Composition

Composition is the primary reuse mechanism.

Objects collaborate through composition.

Avoid artificial inheritance hierarchies.

Prefer

```
Repository

contains

Storage

Validator

Logger

Configuration
```

rather than deeply coupled structures.

---

# Interfaces

Interfaces describe behavior.

They should belong to the consumer.

Avoid defining interfaces prematurely.

Create interfaces only when multiple implementations or testing requirements justify them.

---

# Interface Design

Interfaces should be

- focused
- cohesive
- minimal

Prefer

```
Reader

Writer

Storage
```

Avoid

```
RepositoryManagerServiceInterface
```

Large interfaces indicate poor design.

---

# Dependency Injection

Dependencies should be injected.

Avoid constructing dependencies internally.

Good

```
NewRepository(storage Storage)
```

Poor

```
func NewRepository() {

database := NewDatabase()

...
}
```

Injected dependencies improve

- testing
- modularity
- flexibility

---

# Constructor Design

Constructors should

- establish invariants
- validate inputs
- initialize required dependencies

Avoid optional constructor parameters.

Use configuration structs where appropriate.

---

# Configuration

Configuration belongs in dedicated configuration types.

Avoid scattering configuration throughout the codebase.

Configuration should be immutable after initialization whenever practical.

---

# Cohesion

High cohesion is mandatory.

Each

- package
- struct
- interface
- function

should represent one concept.

Mixed responsibilities indicate poor design.

---

# Coupling

Minimize coupling.

Depend on abstractions.

Avoid importing implementation packages.

Prefer collaboration through interfaces.

---

# Single Responsibility

Every

package

struct

function

should have one reason to change.

Multiple unrelated responsibilities require refactoring.

---

# Open for Extension

Design software that can be extended without modifying unrelated components.

Prefer adding behavior rather than rewriting stable code.

---

# Repository Pattern

Use repositories only when they represent meaningful persistence abstractions.

Avoid repositories that merely wrap another API without adding value.

Repositories should represent business concepts rather than storage engines.

---

# Adapter Pattern

Adapters translate interfaces.

Adapters should not contain business logic.

Their responsibility is translation.

---

# Factory Pattern

Factories should create complex objects.

Do not create factories for simple structs.

Factories exist to reduce construction complexity.

---

# Strategy Pattern

Use strategy objects when behavior varies.

Avoid large conditional chains.

Prefer replacing

```
switch provider
```

with interchangeable implementations when complexity justifies it.

---

# Builder Pattern

Use builders only for complex object construction.

Avoid builders for small immutable structs.

---

# Middleware

Middleware should perform one responsibility.

Examples

- authentication
- logging
- tracing
- metrics
- rate limiting

Avoid middleware containing business logic.

---

# Cross-Cutting Concerns

Separate

- logging
- metrics
- tracing
- authentication
- authorization
- configuration

from business logic.

Cross-cutting concerns should remain reusable.

---

# State Management

Avoid mutable shared state.

Avoid global caches.

Avoid singleton objects.

Prefer explicit ownership.

State should have one owner.

---

# Lifecycle Management

Every long-lived component should have a clearly defined lifecycle.

Initialization

Running

Shutdown

Cleanup

Resources must always be released.

---

# Resource Ownership

Ownership should be explicit.

Every resource should have one owner responsible for cleanup.

Examples

- files
- sockets
- HTTP clients
- database connections
- goroutines

Ownership ambiguity leads to leaks.

---

# Circular Dependencies

Circular dependencies are prohibited.

Refactor shared logic into stable packages.

Package dependency graphs should remain acyclic.

---

# Code Reuse

Reuse existing abstractions before creating new ones.

Avoid duplicated

- validation
- parsing
- serialization
- configuration
- authentication
- authorization

Every duplicated implementation increases maintenance cost.

---

# Simplicity

The simplest correct design is preferred.

Avoid

- unnecessary interfaces
- unnecessary packages
- unnecessary wrappers
- unnecessary abstractions

Every abstraction should justify its existence.

---

# Repository Evolution

Every architectural change should improve

- cohesion
- readability
- modularity
- testability
- consistency
- simplicity

Avoid increasing architectural complexity.

Architecture should continuously improve.

---

# Architectural Review Checklist

Before considering work complete verify

✓ Package responsibilities are clear.

✓ Package boundaries remain clean.

✓ No cyclic dependencies exist.

✓ Existing abstractions were reused.

✓ No duplicate abstractions were introduced.

✓ Interfaces remain minimal.

✓ Dependencies are injected.

✓ Composition is preferred.

✓ Public APIs remain stable.

✓ Internal implementation remains encapsulated.

✓ Domain terminology is consistent.

✓ Cross-cutting concerns remain separated.

✓ Resources have clear ownership.

✓ The architecture is simpler than before the change.

Architecture is considered complete only when it improves the repository rather than merely satisfying the immediate feature request.
