# Load Testing

This directory contains k6 load test scenarios for Promptsheon.

## Prerequisites

1. Install k6: https://k6.io/docs/get-started/installation/
2. Start the Promptsheon server
3. Set environment variables:
   - `K6_BASE_URL` (default: http://localhost:8080)
   - `K6_API_KEY` (optional, for authenticated tests)

## Running Tests

### Run All Tests
```bash
k6 run tests/load/scenarios/*.js
```

### Run Individual Tests
```bash
k6 run tests/load/scenarios/01-health-check.js
k6 run tests/load/scenarios/02-prompt-read.js
k6 run tests/load/scenarios/03-prompt-write.js
# ... etc
```

### Run with Custom Config
```bash
k6 run --config tests/load/k6-config.json tests/load/scenarios/*.js
```

## Test Scenarios

| # | Scenario | Description | VUs | Duration |
|---|----------|-------------|-----|----------|
| 1 | Health Check | Basic health endpoint | 10 | 30s |
| 2 | Prompt Read | List prompts endpoint | 20 | 50s |
| 3 | Prompt Write | Create prompts endpoint | 10 | 50s |
| 4 | Mixed Traffic | 70% read, 30% write | 15 | 60s |
| 5 | Error Injection | Invalid requests | 10 | 30s |
| 6 | Batch Operations | Batch prompt creation | 5 | 30s |
| 7 | Concurrent Reads | High concurrent read load | 25 | 60s |
| 8 | Rate Limiting | Rate limit behavior | 50 | 20s |
| 9 | Large Payloads | Large request payloads | 5 | 30s |
| 10 | Sustained Load | Long-running mixed load | 20 | 100s |

## Interpreting Results

Key metrics to monitor:
- `http_req_duration`: Response time percentiles
- `http_req_failed`: Failed request rate
- `request_success`: Custom success rate
- `http_reqs`: Requests per second

## CI Integration

These tests can be run in CI with:
```bash
k6 run --out json=results.json tests/load/scenarios/*.js
```
