# Promptsheon

**Version Control System for AI Agent Intelligence**

[![CI](https://github.com/sachn-cs/promptsheon/actions/workflows/ci.yaml/badge.svg)](https://github.com/sachn-cs/promptsheon/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sachn-cs/promptsheon)](https://goreportcard.com/report/github.com/sachn-cs/promptsheon)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Promptsheon applies software engineering discipline to AI agents by providing Git-native version control primitives adapted for agent architectures. Every agent state (system prompts, tool definitions, hyperparameters, evaluation metrics) is stored as an immutable, cryptographically-addressed object in a Directed Acyclic Graph (DAG).

## Features

- **Content-Addressable Storage (CAS)** - Immutable, SHA-256-based object storage with Merkle DAG structure
- **Version Control** - Git-like primitives (commit, branch, diff, log) for AI agent configurations
- **Prompt Management** - Create, version, and manage prompts with variable substitution
- **Evaluation Engine** - Run prompts against test datasets with automated scoring
- **LLM Provider Abstraction** - Unified interface for OpenAI, Anthropic, Azure, and Ollama
- **Agent Workflows** - DAG-based workflow execution with tool integration
- **Observability** - Distributed tracing, metrics, and audit logging
- **Guardrails** - Content policy enforcement and safety checks
- **Webhooks** - Event-driven integrations with HMAC signing
- **REST API** - Full-featured HTTP API with OpenAPI specification

## Quick Start

### Prerequisites

- Go 1.23 or later
- SQLite (included via modernc.org/sqlite)

### Installation

```bash
# Clone the repository
git clone https://github.com/sachn-cs/promptsheon.git
cd promptsheon

# Build the server
go build -o promptsheond ./cmd/promptsheond

# Build the CLI
go build -o promptsheon ./cmd/promptsheon
```

### Running the Server

```bash
# Start the server with default configuration
./promptsheond

# Or with custom configuration
PROMPTSHEON_ADDR=:9090 PROMPTSHEON_DB_PATH=./data.db ./promptsheond
```

### Using the CLI

```bash
# Initialize a new repository
./promptsheon init

# Create a prompt
curl -X POST http://localhost:8080/api/v1/prompts \
  -H "Content-Type: application/json" \
  -d '{"name":"greeting","content":"Hello {{name}}, welcome to {{product}}!"}'

# Run a prompt
curl -X POST http://localhost:8080/api/v1/prompts/{id}/run \
  -H "Content-Type: application/json" \
  -d '{"variables":{"name":"World","product":"Promptsheon"}}'
```

## Architecture

### Storage Model

Promptsheon uses four object types organized in a Merkle DAG:

- **Blob**: Raw content (prompts, configs, tool specs)
- **Tree**: A named mapping of blobs forming a configuration snapshot
- **Commit**: An immutable node referencing a tree, with parent history and metadata
- **Ref**: A mutable pointer (branch) to a named commit

### Project Structure

```
promptsheon/
├── cmd/
│   ├── promptsheon/      # CLI client
│   └── promptsheond/     # Server daemon
├── internal/
│   ├── api/              # HTTP REST API
│   ├── llm/              # LLM provider abstraction
│   ├── models/           # Domain models
│   ├── store/            # SQLite persistence
│   ├── eval/             # Evaluation engine
│   ├── trace/            # Distributed tracing
│   ├── metrics/          # Prometheus metrics
│   └── ...               # Other packages
├── sdk/                  # Go client SDK
├── api/                  # OpenAPI specification
└── test/                 # Integration tests
```

## API Documentation

The full API specification is available in [api/openapi.yaml](api/openapi.yaml).

### Key Endpoints

| Method | Endpoint                      | Description             |
| ------ | ----------------------------- | ----------------------- |
| `GET`  | `/health`                     | Health check            |
| `POST` | `/api/v1/prompts`             | Create a prompt         |
| `GET`  | `/api/v1/prompts`             | List prompts            |
| `POST` | `/api/v1/prompts/{id}/run`    | Execute a prompt        |
| `POST` | `/api/v1/prompts/{id}/stream` | Stream prompt execution |
| `POST` | `/api/v1/eval/run`            | Run evaluation          |
| `GET`  | `/api/v1/traces`              | Query traces            |
| `GET`  | `/api/v1/metrics`             | Get metrics             |

## Configuration

Promptsheon can be configured via environment variables:

| Variable                | Default          | Description                          |
| ----------------------- | ---------------- | ------------------------------------ |
| `PROMPTSHEON_ADDR`      | `:8080`          | Server listen address                |
| `PROMPTSHEON_DB_PATH`   | `promptsheon.db` | SQLite database path                 |
| `PROMPTSHEON_AUTH`      | `false`          | Enable authentication                |
| `PROMPTSHEON_LOG_LEVEL` | `info`           | Log level (debug, info, warn, error) |
| `PROMPTSHEON_VAULT_KEY` | -                | Encryption key for provider API keys |

### LLM Provider Configuration

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Azure OpenAI
export AZURE_OPENAI_API_KEY="..."
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com/"

# Ollama (local)
export OLLAMA_BASE_URL="http://localhost:11434"
```

## Development

### Prerequisites

- Go 1.23+
- golangci-lint (for linting)

### Building

```bash
make build
```

### Testing

```bash
# Run all tests
make test

# Run tests with race detection
go test -race ./...

# Run integration tests
go test -v ./test/...
```

### Linting

```bash
make lint
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/sachn-cs/promptsheon/issues)
- **Documentation**: [docs/](docs/)
- **API Spec**: [api/openapi.yaml](api/openapi.yaml)

## Acknowledgments

Built with Go and inspired by Git's immutable object model.
