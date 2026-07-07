# Promptsheon

**Version Control System for AI Agent Intelligence**

[![CI](https://github.com/sachncs/promptsheon/actions/workflows/ci.yaml/badge.svg)](https://github.com/sachncs/promptsheon/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sachncs/promptsheon)](https://goreportcard.com/report/github.com/sachncs/promptsheon)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Promptsheon applies software engineering discipline to AI agents by providing Git-native version control primitives adapted for agent architectures. Every agent state (system prompts, tool definitions, hyperparameters, evaluation metrics) is stored as an immutable, cryptographically-addressed object in a Directed Acyclic Graph (DAG).

## Quick start

```bash
git clone https://github.com/sachncs/promptsheon.git
cd promptsheon

# Build both binaries
go build -o promptsheond ./cmd/promptsheond
go build -o promptsheon  ./cmd/promptsheon

# Start the server (default :8080, promptsheon.db)
./promptsheond

# Create a prompt
curl -X POST http://localhost:8080/api/v1/prompts \
  -H "Content-Type: application/json" \
  -d '{"name":"greeting","content":"Hello {{name}}, welcome to {{product}}!"}'

# Run it
curl -X POST http://localhost:8080/api/v1/prompts/{id}/run \
  -H "Content-Type: application/json" \
  -d '{"variables":{"name":"World","product":"Promptsheon"}}'
```

## Features

- **Content-Addressable Storage (CAS)** — immutable, SHA-256-based object storage with Merkle DAG structure
- **Version control** — Git-like primitives (commit, branch, diff, log) for AI agent configurations
- **Prompt management** — create, version, and manage prompts with variable substitution
- **Evaluation engine** — run prompts against test datasets with automated scoring
- **LLM provider abstraction** — unified interface for OpenAI, Anthropic, Azure OpenAI, Ollama, NVIDIA NIM
- **Agent workflows** — DAG-based workflow execution with tool integration
- **Observability** — distributed tracing, metrics, and audit logging
- **Guardrails** — content policy enforcement and safety checks
- **Webhooks** — event-driven integrations with HMAC signing and SSRF protection
- **REST API** — full-featured HTTP API with generated OpenAPI specification

## Documentation

The full documentation is at **[docs/](docs/)**. Start with:

- [Getting Started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [API Reference](docs/api-reference.md) and the [OpenAPI spec](api/openapi.yaml)
- [Architecture](docs/architecture.md) and [Modules](docs/modules.md)
- [Security](docs/security.md) and [Design Decisions](docs/design-decisions.md)
- [Troubleshooting](docs/troubleshooting.md) and [FAQ](docs/faq.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/development.md](docs/development.md).

## Security

See [SECURITY.md](SECURITY.md). Report vulnerabilities to the GitHub Security Advisories workflow — do not open a public issue.

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Support

- **Issues**: [GitHub Issues](https://github.com/sachncs/promptsheon/issues)
- **Discussions**: [GitHub Discussions](https://github.com/sachncs/promptsheon/discussions)
