# Evaluations

The evaluation engine runs prompts against test datasets, scores outputs, and detects hallucinations. It provides quantitative metrics for comparing prompt versions and models.

## Concepts

### Test Dataset

A named collection of test cases, each defining inputs and expected outputs:

```json
{
  "name": "greeting-eval",
  "cases": [
    {
      "id": "tc-1",
      "input": {"name": "Alice", "product": "Promptsheon"},
      "expected_output": "Hello Alice, welcome to Promptsheon!",
      "tags": ["greeting", "happy-path"]
    },
    {
      "id": "tc-2",
      "input": {"name": "Bob"},
      "expected_contains": ["Bob", "Promptsheon"],
      "tags": ["greeting", "default-product"]
    }
  ]
}
```

| Field | Description |
|---|---|
| `input` | Variable values substituted into the prompt template |
| `expected_output` | Exact match (case-insensitive) |
| `expected_contains` | Substring checks (case-insensitive) |
| `tags` | Labels for filtering and grouping |

### Scoring

Each test case output is scored 0.0 to 1.0:

- **Exact match**: 1.0 if `expected_output` matches, 0.0 otherwise
- **Contains checks**: 1.0 per matching substring in `expected_contains`
- **Final score**: Average across all checks
- **No expectations**: Score defaults to 1.0 (pass)

Custom scorers can be registered via the `Scorer` interface.

### Hallucination Detection

The `HallucinationDetector` uses an LLM to evaluate whether the output contains fabricated information not supported by the prompt or input context. It returns a score from 0.0 (no hallucination) to 1.0 (heavy hallucination).

### Aggregate Metrics

| Metric | Description |
|---|---|
| `total_cases` | Number of test cases evaluated |
| `passed_cases` | Cases with score >= 0.5 |
| `pass_rate` | `passed_cases / total_cases` |
| `avg_score` | Mean score across all cases |
| `avg_latency_ms` | Mean LLM response latency |
| `avg_hallucination` | Mean hallucination score |
| `total_tokens` | Sum of all token usage |

## API Usage

### Run an Evaluation

```bash
curl -X POST http://localhost:8080/api/v1/eval/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt_hash": "sha256:abc123...",
    "dataset_id": "dataset-001",
    "model": "gpt-4o"
  }'
```

The system looks up the prompt by its CAS hash and runs it against every test case in the dataset.

### Get Results

```bash
# Filter by prompt hash
curl "http://localhost:8080/api/v1/eval/results?prompt_hash=sha256:abc123..."

# Filter by dataset
curl "http://localhost:8080/api/v1/eval/results?dataset_id=dataset-001"
```

### Get a Report

```bash
curl "http://localhost:8080/api/v1/eval/report?prompt_hash=sha256:abc123..."
```

Returns full results with aggregate metrics.

### Compare Evaluations

```bash
curl "http://localhost:8080/api/v1/eval/compare?run_a=run-001&run_b=run-002"
```

Compare metrics between two evaluation runs to see improvement or regression.

## Integration with CI

```bash
# Run eval and check pass rate
REPORT=$(curl -s -X POST http://localhost:8080/api/v1/eval/run \
  -H "Content-Type: application/json" \
  -d '{"prompt_hash":"...","dataset_id":"...","model":"gpt-4o"}')

PASS_RATE=$(echo "$REPORT" | jq '.aggregate.pass_rate')
if (( $(echo "$PASS_RATE < 0.8" | bc -l) )); then
  echo "FAIL: pass rate $PASS_RATE < 0.8"
  exit 1
fi
```
