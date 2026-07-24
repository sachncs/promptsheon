.PHONY: all build build-server build-cli test test-verbose test-integration test-e2e load-test lint lint-domain lint-deps fmt vet deps clean coverage coverage-raw run cli openapi openapi-check update-deps security helm-docs docs-check bench docs-site help

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
	go test -v -race -count=1 ./tests/...

# Run end-to-end tests (HTTP API against a built daemon)
test-e2e:
	go test -v -race -count=1 -timeout 300s ./tests/e2e/...

# Run k6 load scenarios against a running daemon. Set
# PROMPTSHEON_ADDR to override the target URL; the default is
# http://localhost:8080.
load-test:
	@command -v k6 >/dev/null 2>&1 || { echo "k6 not installed (brew install k6)"; exit 1; }
	@mkdir -p /tmp/load-results
	@for scenario in tests/load/scenarios/*.js; do \
	  name=$$(basename "$$scenario" .js); \
	  echo "=== $$name ==="; \
	  k6 run "$$scenario" --out json=/tmp/load-results/$$name.json || true; \
	done

# Run linter
lint:
	golangci-lint run

# Lint domain packages: fail on any package-level mutable state
# (Charter Principle 5). Runs as part of CI. The check is a small
# AST walker at scripts/check-no-package-state.go; it allows error
# sentinels and import-pin discards.
lint-domain:
	go run ./scripts/check-no-package-state.go

# Lint domain-purity: fail if any domain package imports internal/llm,
# internal/api, internal/store, or cmd. Domain packages depend only
# on each other and the standard library (Charter Principle 5).
lint-deps:
	scripts/check-domain-purity.sh

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

# Regenerate the Helm chart's README.md from values.yaml. Requires
# the helm-docs binary; if absent the target is a no-op. CI runs
# this on a tag and commits the regenerated README.
helm-docs:
	@command -v helm-docs >/dev/null 2>&1 || { echo "helm-docs not installed (brew install helm-docs)"; exit 0; }
	helm-docs --sort-values-order=file -t deploy/helm/promptsheon/README.md deploy/helm/promptsheon

# DOC-CI-3 / DOC-FRESH-1: deterministic doc-freshness check.
# Walks every markdown file under docs/ plus the root
# README.md / CHANGELOG.md and reports:
#   - any local markdown link that resolves to a missing file
#     (HTTP(S), mailto, and pure-anchor links are skipped)
#   - any path-shaped reference to source code
#     (internal/...go, pkg/...go, cmd/...go, api/openapi.yaml,
#     deploy/, scripts/, .github/workflows/, docs/adr/NNNN-...)
#     that no longer points at a real file
# Pure shell + awk; no new Go dependency. CI runs this on
# every PR. Failing here means a documented reference is
# stale or a local link is broken.
docs-check:
	bash scripts/docs-check.sh

# PERF-BENCH-1: curated Go benchmark target. The list in
# scripts/benchmarks.txt is the canonical eight trustworthy
# benchmarks. `go test -bench` reports ns/op and B/op; the
# p99 latency gate lives in the k6 scenarios, not here.
# Override the per-benchmark time with BENCHTIME=100ms for
# a fast smoke pass.
bench:
	@BENCHTIME="$(or $(BENCHTIME),1s)" bash scripts/run-benchmarks.sh

# Build the mdBook site. The book lives in docs-site/ and reuses
# docs/ as its source via the relative `src = "../docs"` in
# docs-site/book.toml. The only mdBook-only file is
# docs/SUMMARY.md; no markdown is duplicated. Requires mdbook
# on PATH; if absent the target is a no-op (CI installs mdbook
# via the docs-site.yaml workflow).
docs-site:
	@command -v mdbook >/dev/null 2>&1 || { echo "mdbook not installed (cargo install mdbook)"; exit 0; }
	mdbook build docs-site

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
	@echo "  helm-docs        Regenerate deploy/helm/promptsheon/README.md from values.yaml"
	@echo "  docs-check       Fail on broken local markdown links or stale source-path refs"
	@echo "  bench            Run the curated 8 Go benchmarks (scripts/benchmarks.txt)"
	@echo "  docs-site        Build the mdBook site (no-op if mdbook is missing)"
	@echo "  update-deps      Update all dependencies"
	@echo "  security         Check for security vulnerabilities"
	@echo "  help             Show this help message"
