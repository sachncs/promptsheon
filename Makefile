.PHONY: all build build-server build-cli test lint lint-domain clean help

# Default target
all: build

# Build all binaries
build: build-server build-cli

# Build the server daemon
build-server:
	go build -o promptsheond ./cmd/promptsheond

# Build the CLI client
build-cli:
	go build -o promptsheon ./cmd/promptsheon

# Run all tests
test:
	go test -race -count=1 ./...

# Run tests with verbose output
test-verbose:
	go test -v -race -count=1 ./...

# Run integration tests
test-integration:
	go test -v -race -count=1 ./test/...

# Run linter
lint:
	golangci-lint run

# Lint domain packages: fail on any package-level mutable state
# (Charter Principle 5). Runs as part of CI. The check is a small
# AST walker at scripts/check-no-package-state.go; it allows error
# sentinels and import-pin discards.
lint-domain:
	go run ./scripts/check-no-package-state.go

# Format code
fmt:
	gofmt -s -w .
	goimports -w .

# Run go vet
vet:
	go vet ./...

# Download dependencies
deps:
	go mod download
	go mod verify

# Clean build artifacts
clean:
	rm -f promptsheon promptsheond
	rm -f *.db *.db-journal *.db-wal *.db-shm
	rm -f coverage.out coverage.html

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Show coverage in terminal
coverage-raw:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run the server locally
run:
	go run ./cmd/promptsheond

# Run the CLI
cli:
	go run ./cmd/promptsheon

# Regenerate api/openapi.yaml from the server's route
# registrations. The generator parses internal/api/server.go
# for routes and internal/api/handlers_*.go for request
# schemas, then emits a real OpenAPI 3.0 spec. Re-run this
# target whenever a route or handler changes.
openapi:
	go run ./scripts/genopenapi

# Check that api/openapi.yaml is up to date. CI runs this
# target and fails the build if a developer added a route
# without regenerating the spec.
openapi-check: openapi
	@git diff --exit-code api/openapi.yaml || (echo "api/openapi.yaml is out of date. Run 'make openapi' and commit the result."; exit 1)

# Update dependencies
update-deps:
	go get -u ./...
	go mod tidy

# Check for security vulnerabilities
security:
	govulncheck ./...

# Show help
help:
	@echo "Promptsheon - Version Control for AI Intelligence"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              Build all binaries (default)"
	@echo "  build            Build all binaries"
	@echo "  build-server     Build the server daemon"
	@echo "  build-cli        Build the CLI client"
	@echo "  test             Run all tests with race detection"
	@echo "  test-verbose     Run tests with verbose output"
	@echo "  test-integration Run integration tests"
	@echo "  lint             Run golangci-lint"
	@echo "  lint-domain      Fail on package-level mutable state in domain packages"
	@echo "  fmt              Format code with gofmt and goimports"
	@echo "  vet              Run go vet"
	@echo "  deps             Download and verify dependencies"
	@echo "  clean            Remove build artifacts"
	@echo "  coverage         Generate HTML coverage report"
	@echo "  coverage-raw     Show coverage in terminal"
	@echo "  run              Run the server locally"
	@echo "  cli              Run the CLI"
	@echo "  openapi          Regenerate api/openapi.yaml from server routes"
	@echo "  openapi-check    Fail if openapi.yaml is out of date"
	@echo "  update-deps      Update all dependencies"
	@echo "  security         Check for security vulnerabilities"
	@echo "  help             Show this help message"
