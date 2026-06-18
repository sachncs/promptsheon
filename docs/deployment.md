# Deployment

## Production Build

```bash
# Build optimized binaries
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o promptsheond ./cmd/promptsheond
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o promptsheon ./cmd/promptsheon
```

## Systemd Service

Create `/etc/systemd/system/promptsheon.service`:

```ini
[Unit]
Description=Promptsheon Server
After=network.target

[Service]
Type=simple
User=promptsheon
Group=promptsheon
ExecStart=/usr/local/bin/promptsheond
Restart=on-failure
RestartSec=5

Environment=PROMPTSHEON_ADDR=:8080
Environment=PROMPTSHEON_DB_PATH=/var/lib/promptsheon/promptsheon.db
Environment=PROMPTSHEON_AUTH=true
Environment=PROMPTSHEON_LOG_LEVEL=info
Environment=PROMPTSHEON_VAULT_KEY=your-vault-key-here
Environment=OPENAI_API_KEY=sk-...

ProtectSystem=strict
ReadWritePaths=/var/lib/promptsheon
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable promptsheon
sudo systemctl start promptsheon
```

## Docker

Create `Dockerfile`:

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o promptsheond ./cmd/promptsheond

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/promptsheond /usr/local/bin/
RUN adduser -D promptsheon
USER promptsheon
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["promptsheond"]
ENV PROMPTSHEON_DB_PATH=/data/promptsheon.db
```

```bash
# Build
docker build -t promptsheon:latest .

# Run
docker run -d \
  --name promptsheon \
  -p 8080:8080 \
  -v promptsheon-data:/data \
  -e PROMPTSHEON_AUTH=true \
  -e PROMPTSHEON_VAULT_KEY=your-key \
  -e OPENAI_API_KEY=sk-... \
  promptsheon:latest
```

## Docker Compose

```yaml
version: "3.8"
services:
  promptsheon:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - promptsheon-data:/data
    environment:
      PROMPTSHEON_DB_PATH: /data/promptsheon.db
      PROMPTSHEON_AUTH: "true"
      PROMPTSHEON_VAULT_KEY: ${VAULT_KEY}
      OPENAI_API_KEY: ${OPENAI_API_KEY}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  promptsheon-data:
```

## Reverse Proxy (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name promptsheon.example.com;

    ssl_certificate     /etc/ssl/certs/promptsheon.pem;
    ssl_certificate_key /etc/ssl/private/promptsheon.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # SSE streaming endpoint
    location /api/v1/logs/stream {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
    }
}
```

## Health Checks

Use the health and readiness endpoints for load balancer configuration:

| Endpoint | Purpose | Response |
|---|---|---|
| `GET /health` | Liveness — is the server running? | `200 {"status":"ok"}` |
| `GET /ready` | Readiness — is the database accessible? | `200` when ready |

## Backup

SQLite database backup:

```bash
# Hot backup (safe while server is running)
sqlite3 promptsheon.db ".backup backup.db"

# Or stop server, copy, restart
cp promptsheon.db promptsheon.db.bak
```

## Monitoring

### Prometheus

Scrape the metrics endpoint:

```yaml
scrape_configs:
  - job_name: "promptsheon"
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: "/metrics"
```

Key metrics to monitor:

| Metric | Description |
|---|---|
| `http_requests_total` | Total HTTP requests by endpoint |
| `http_request_duration_seconds` | Request latency histogram |
| `llm_calls_total` | Total LLM provider calls |
| `llm_call_duration_seconds` | LLM call latency |
| `guardrail_violations_total` | Total guardrail violations |
| `eval_runs_total` | Total evaluation runs |

### Log Aggregation

Logs are emitted as JSON to stderr. Pipe to your log aggregator:

```bash
./promptsheond 2>&1 | jq .
```

Log fields: `time`, `level`, `msg`, `err`, `method`, `path`.
