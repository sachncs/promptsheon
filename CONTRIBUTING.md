# Contributing to Promptsheon

Thank you for your interest in contributing to Promptsheon! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Style Guidelines](#style-guidelines)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.23 or later
- Git
- golangci-lint (for linting)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:

   ```bash
   git clone https://github.com/your-username/promptsheon.git
   cd promptsheon
   ```

3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/sachn-cs/promptsheon.git
   ```

## Development Setup

### Build

```bash
# Build all binaries
go build ./...

# Or use Make
make build
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run integration tests
go test -v ./test/...
```

### Lint

```bash
# Run linter
golangci-lint run

# Or use Make
make lint
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `feature/add-new-scorer`
- `fix/handle-nil-pointer`
- `docs/update-readme`
- `refactor/improve-error-handling`

### Commit Messages

Write clear, concise commit messages:

- Use the imperative mood ("Add feature" not "Added feature")
- Keep the first line under 72 characters
- Reference issues and pull requests when relevant

Examples:

```
Add exact match scorer for evaluation

- Implements ExactMatchScorer
- Adds unit tests
- Updates documentation

Closes #42
```

### Code Style

Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) and [Effective Go](https://golang.org/doc/effective_go.html) guidelines.

Key points:

- Use `gofmt` to format code
- Follow naming conventions (exported names are CamelCase)
- Write meaningful variable and function names
- Add comments for exported types and functions
- Handle errors explicitly

## Testing

### Unit Tests

- Place tests in `_test.go` files alongside the code
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for meaningful coverage, not just high percentages

Example:

```go
func TestExactMatchScorer_Score(t *testing.T) {
    tests := []struct {
        name     string
        expected string
        actual   string
        want     float64
    }{
        {
            name:     "exact match",
            expected: "hello",
            actual:   "hello",
            want:     1.0,
        },
        {
            name:     "no match",
            expected: "hello",
            actual:   "world",
            want:     0.0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            s := ExactMatchScorer{}
            got := s.Score(tt.expected, tt.actual)
            if got != tt.want {
                t.Errorf("Score() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

- Place integration tests in `test/` directory
- Use `httptest.NewServer` for API tests
- Clean up resources with `t.Cleanup`

## Pull Request Process

1. **Create a feature branch** from `main`
2. **Make your changes** with clear commits
3. **Write or update tests** for your changes
4. **Run the full test suite**:
   ```bash
   go test -race ./...
   golangci-lint run
   ```
5. **Push your branch** and create a pull request
6. **Fill out the PR template** completely
7. **Wait for CI** to pass
8. **Request review** from maintainers

### PR Checklist

- [ ] Code compiles without errors
- [ ] All tests pass
- [ ] Linter passes
- [ ] New code has tests
- [ ] Documentation is updated (if applicable)
- [ ] Commit messages are clear
- [ ] PR description explains the changes

## Style Guidelines

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` and `goimports`
- Keep functions focused and small
- Prefer composition over inheritance
- Use interfaces to define contracts

### Documentation

- Add GoDoc comments to all exported types and functions
- Use complete sentences in comments
- Include examples where helpful
- Keep README.md updated

### Error Handling

- Always check and handle errors
- Use `fmt.Errorf` with `%w` for error wrapping
- Create meaningful error messages
- Consider using custom error types for domain errors

## Reporting Issues

### Bug Reports

Include:

- Go version (`go version`)
- Operating system and architecture
- Steps to reproduce
- Expected behavior
- Actual behavior
- Relevant logs or error messages

### Feature Requests

Include:

- Clear description of the feature
- Use case / motivation
- Proposed implementation (if any)
- Alternatives considered

## Questions?

- Open a [GitHub Discussion](https://github.com/sachn-cs/promptsheon/discussions)
- Check existing [issues](https://github.com/sachn-cs/promptsheon/issues)

Thank you for contributing!
