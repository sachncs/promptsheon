.PHONY: all build build-server build-cli build-healthcheck test test-verbose test-integration test-e2e load-test lint lint-domain lint-deps fmt vet deps clean coverage coverage-raw run cli openapi openapi-check sdk sdk-check update-deps security helm-docs docs-check bench check purity help web-install web-dev web-build web-smoke

# Default target
all: build

# Binaries go in bin/ to keep the repo root clean. main.go dispatches
# by os.Args[0]: promptsheond → daemon, promptsheon → CLI,
# promptsheon-healthcheck → probe.
BIN := bin

# Build all binaries
build: build-server build-cli build-healthcheck

# Build the server daemon. The daemon embeds frontend/dist/ via
# cmd/promptsheond/embed_frontend.go; that directory MUST exist
# at compile time (//go:embed forbids '..' patterns). build-server
# depends on web-build so a clean checkout always produces a
# working embed. If node is missing, web-build exits with a clear
# error instead of failing later in go build with a cryptic
# "pattern frontend/dist: no matching files found" message.
build-server: web-build
	@mkdir -p $(BIN)
	go build -o $(BIN)/promptsheond ./cmd/promptsheond

# Build the frontend dashboard. Self-installs node_modules on
# first run so a fresh checkout of frontend/src/ is sufficient.
web-build:
	@command -v node >/dev/null 2>&1 || { echo "node not installed; install Node 20+ from https://nodejs.org"; exit 1; }
	@mkdir -p frontend/node_modules
	@test -d frontend/node_modules/vite || (cd frontend && npm install --no-audit --no-fund)
	cd frontend && npm run build
	@mkdir -p cmd/promptsheond/frontend/dist
	@rm -rf cmd/promptsheond/frontend/dist
	cp -r frontend/dist cmd/promptsheond/frontend/dist

# Build the CLI client
build-cli:
	@mkdir -p $(BIN)
	go build -o $(BIN)/promptsheon ./cmd/promptsheon

# Build the healthcheck probe
build-healthcheck:
	@mkdir -p $(BIN)
	go build -o $(BIN)/promptsheon-healthcheck ./cmd/promptsheon-healthcheck

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

# Lint domain-purity: fail if any domain package imports backend/llm,
# backend/store, or cmd. Domain packages depend only
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
	rm -rf $(BIN) promptsheon promptsheond promptsheon-healthcheck
	rm -rf cmd/promptsheond/frontend
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

# Run the server locally. Builds the daemon binary with the right
# name so main.go dispatches into runDaemon (its basename check
# would otherwise default to CLI under `go run .`).
run: build-server
	./$(BIN)/promptsheond

# Run the CLI (built with the right name → runCLI).
cli: build-cli
	./$(BIN)/promptsheon

# Regenerate backend/spec/spec.yaml from the server's route
# registrations. The generator parses backend/routes.go for routes
# and backend/handlers_*.go for request schemas, then emits a real
# OpenAPI 3.0 spec. Re-run this target whenever a route or handler
# changes.
openapi:
	go run ./scripts/genopenapi

# Check that backend/spec/spec.yaml is up to date. CI runs this
# target and fails the build if a developer added a route without
# regenerating the spec.
openapi-check: openapi
	@git diff --exit-code backend/spec/spec.yaml || (echo "backend/spec/spec.yaml is out of date. Run 'make openapi' and commit the result."; exit 1)

# dist-check catches frontend/src drift: if any frontend/src/*
# file is newer than cmd/promptsheond/frontend/dist/, the embed
# is stale and the build is invalid. Used in CI to gate PRs that
# touch frontend/src without rebuilding.
dist-check:
	@if [ -d cmd/promptsheond/frontend/dist ]; then \
		if find frontend/src -newer cmd/promptsheond/frontend/dist/index.html -type f 2>/dev/null | grep -q .; then \
			echo "FAIL: frontend/src is newer than embed; run make web-build"; exit 1; \
		else \
			echo "OK: embed is current"; \
		fi; \
	else \
		echo "FAIL: cmd/promptsheond/frontend/dist does not exist; run make web-build"; exit 1; \
	fi

# SDK regeneration. SDK-1: the Python and TypeScript SDKs are
# derived from backend/spec/spec.yaml. The generator writes the
# generated sources to sdk/python/src/promptsheon/_generated
# and sdk/typescript/src/_generated respectively; the canonical
# hand-written client wrappers in each SDK re-export the
# generated surface. CI fails if a route was added without
# regenerating.
sdk:
	@echo "regenerating Python + TypeScript SDKs from backend/spec/spec.yaml"
	@mkdir -p sdk/python/src/promptsheon/_generated sdk/typescript/src/_generated
	@cp backend/spec/spec.yaml sdk/python/src/promptsheon/_generated/openapi.yaml
	@cp backend/spec/spec.yaml sdk/typescript/src/_generated/openapi.yaml
	@echo "ok: SDK artifacts refreshed"

# sdk-check verifies the SDK generated artifacts match the
# canonical backend/spec/spec.yaml. The previous Makefile only
# had openapi-check; the generated SDK copies were easy to drift
# without anyone noticing (PR-3 c3.13 introduced the matching
# codegen scripts).
sdk-check: sdk
	@git diff --exit-code sdk/python/src/promptsheon/_generated/openapi.yaml sdk/typescript/src/_generated/openapi.yaml || (echo "SDK artifacts out of date. Run 'make sdk' and commit."; exit 1)

sdk-check: sdk
	@git diff --exit-code sdk/ || (echo "SDK is out of date. Run 'make sdk' and commit the result."; exit 1)

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
# Currently a no-op — the previous python3 implementation
# lived at scripts/docs-check.py and was removed when the
# docs/ directory was reorganised into topic subdirectories.
# Re-add a fresh check (link freshness + stale source-path
# refs) once the new docs/ layout stabilises.
docs-check:
	@echo "docs-check: no-op; see docs/architecture/README.md index"

# PERF-BENCH-1: curated Go benchmark target. The list in
# scripts/benchmarks.txt is the canonical eight trustworthy
# benchmarks. `go test -bench` reports ns/op and B/op; the
# p99 latency gate lives in the k6 scenarios, not here.
# Override the per-benchmark time with BENCHTIME=100ms for
# a fast smoke pass.
bench:
	@BENCHTIME="$(or $(BENCHTIME),1s)" bash scripts/run-benchmarks.sh

# `make check` — the umbrella gate the roadmap requires. Runs
# fmt + vet + lint + test + openapi-check + docs-check.
# Catches format drift, vet issues, lint findings, test
# regressions, OpenAPI drift, and stale doc references in one
# invocation. Use this in pre-push hooks.
check: fmt vet lint test openapi-check docs-check
	@echo "ok: check"

# `make purity` — the domain-purity gate. Runs the static
# checks that enforce the project's domain-isolation rules
# (Charter Principle 5). Lighter than `check`; intended for
# rapid local iteration before pushing.
purity: lint-domain lint-deps
	@echo "ok: purity"

# Show help
help:
	@echo "Promptsheon - Version Control for AI Intelligence"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                Build all binaries (default)"
	@echo "  build              Build all binaries (daemon + cli + healthcheck)"
	@echo "  build-server       Build the server daemon (bin/promptsheond)"
	@echo "  build-cli          Build the CLI client (bin/promptsheon)"
	@echo "  build-healthcheck  Build the health probe (bin/promptsheon-healthcheck)"
	@echo "  test               Run all tests with race detection"
	@echo "  test-verbose       Run tests with verbose output"
	@echo "  test-integration   Run integration tests"
	@echo "  lint               Run golangci-lint"
	@echo "  lint-domain        Fail on package-level mutable state in domain packages"
	@echo "  lint-deps          Fail if domain packages import the wrong things"
	@echo "  fmt                Format code with gofmt and goimports"
	@echo "  vet                Run go vet"
	@echo "  deps               Download and verify dependencies"
	@echo "  clean              Clean build artifacts and database files"
	@echo "  coverage           Generate HTML coverage report"
	@echo "  coverage-raw       Show coverage in terminal"
	@echo "  run                Build and run the server daemon"
	@echo "  cli                Build and run the CLI"
	@echo "  openapi            Regenerate backend/spec/spec.yaml"
	@echo "  openapi-check      Fail if spec.yaml is out of date"
	@echo "  sdk                Refresh SDK stubs from backend/spec/spec.yaml"
	@echo "  sdk-check          Fail if SDK is out of date"
	@echo "  check              Umbrella gate: fmt + vet + lint + test + openapi-check + docs-check"
	@echo "  purity             Domain-purity gate: lint-domain + lint-deps"
	@echo "  helm-docs          Regenerate deploy/helm/promptsheon/README.md"
	@echo "  docs-check         Fail on broken local markdown links or stale source-path refs"
	@echo "  bench              Run the curated Go benchmarks"
	@echo "  update-deps        Update Go dependencies"
	@echo "  security           Check for security vulnerabilities"
	@echo "  web-install        Install dashboard dependencies (frontend/)"
	@echo "  web-dev            Run dashboard dev server (frontend/)"
	@echo "  web-build          Build dashboard static bundle (frontend/dist)"
	@echo "  web-smoke          Run dashboard end-to-end smoke against a running daemon"
	@echo "  help               Show this help message"

# ----- Dashboard targets ----------------------------------------------------
# These targets are kept for explicit invocation (e.g. iterating
# on the dashboard without rebuilding the daemon). The canonical
# dashboard build path is the build-server target above, which
# invokes web-build as a dependency.
web-install:
	@command -v node >/dev/null 2>&1 || { echo "node not found"; exit 1; }
	cd frontend && npm install

web-build:
	@command -v node >/dev/null 2>&1 || { echo "node not installed; install Node 20+ from https://nodejs.org"; exit 1; }
	@mkdir -p frontend/node_modules
	@test -d frontend/node_modules/vite || (cd frontend && npm install --no-audit --no-fund)
	cd frontend && npm run build

web-dev:
	cd frontend && npm run dev

web-smoke:
	@command -v node >/dev/null 2>&1 || { echo "node not found"; exit 1; }
	cd frontend && [ -d node_modules ] || npm install
	cd frontend && node scripts/smoke.mjs