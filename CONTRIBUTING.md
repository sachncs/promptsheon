# Contributing to Promptsheon

Thank you for your interest in contributing!

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting started](#getting-started)
- [Pull request process](#pull-request-process)
- [Reporting issues](#reporting-issues)
- [Questions](#questions)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting started

1. Read [docs/development.md](docs/development.md) for setup, repository layout, code style, Make targets, the OpenAPI generator workflow, and migration conventions.
2. Read [docs/architecture.md](docs/architecture.md) for the system diagram and request lifecycle.
3. Read [docs/modules.md](docs/modules.md) to find the package closest to the area you want to work on.
4. Read [docs/testing.md](docs/testing.md) for the test taxonomy and helpers.

## Pull request process

1. Create a feature branch from `main` (`feature/<slug>`, `fix/<slug>`, `docs/<slug>`, `refactor/<slug>`).
2. Make your changes with clear commits. Use the imperative mood and keep the first line under 72 characters.
3. Write or update tests for your changes. See [docs/testing.md](docs/testing.md).
4. Run the full pre-PR checklist:

   ```bash
   go test -race -count=1 ./...
   make lint
   make openapi-check
   ```

5. Push your branch and open a pull request. Fill out the PR template completely.
6. Wait for CI to pass and request a review.

If your change is documentation-only, the OpenAPI step is a no-op and can be skipped. If your change touches a route or handler, `make openapi-check` will fail the build if the spec is out of date — regenerate and commit.

## Style guidelines

- Follow [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- `gofmt -s` and `goimports` are mandatory. CI rejects PRs with unformatted code.
- Every exported type and function must have a GoDoc comment.
- Tests are table-driven where it makes sense. Use `t.Run` subtests.
- Error wrapping: `fmt.Errorf("context: %w", err)`.

## Reporting issues

Include:

- Go version (`go version`)
- Operating system and architecture
- Steps to reproduce
- Expected behaviour
- Actual behaviour
- Relevant logs (`./promptsheond 2>&1 | jq .`)

For security vulnerabilities, **do not** open a public issue. See [SECURITY.md](SECURITY.md).

## Questions

- Open a [GitHub Discussion](https://github.com/sachn-cs/promptsheon/discussions)
- Check existing [issues](https://github.com/sachn-cs/promptsheon/issues)
- The [docs/FAQ.md](docs/faq.md) and [docs/glossary.md](docs/glossary.md)
