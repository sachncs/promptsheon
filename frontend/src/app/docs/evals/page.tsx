import { DocPage, DocNext, DocCurl } from '@/components/brand/doc-page';

export const metadata = {
  title: 'Evaluation engine · Promptsheon',
};

export default function EvalsPage() {
  return (
    <DocPage
      title="Evaluation engine"
      subtitle="Versioned suites, deterministic graders, pass@k statistics, human-review queue, and calibration report."
    >
      <h2>Suite model</h2>
      <p>An <code>EvalSuite</code> is keyed to a capability. Each suite has one or more versions; a version
        carries its own <code>graderConfig</code>, <code>passThreshold</code>, <code>borderlineBand</code>,
        and the k/n pair used for pass@k.</p>

      <h2>Graders</h2>
      <ul>
        <li><strong>regex_match</strong> — pattern + flags, applied to a field.</li>
        <li><strong>schema_state_check</strong> — JSON-schema + optional jq over the final state.</li>
        <li><strong>tool_call_assertion</strong> — sequence-of-calls expectations.</li>
        <li><strong>transcript_diff</strong> — diff the run against a reference transcript.</li>
        <li><strong>llm_rubric</strong> — anchored rubric, evaluated by the LLM agent.</li>
      </ul>

      <h2>Routes</h2>
      <DocCurl cmd="POST /api/eval-suites" />
      <DocCurl cmd="POST /api/eval-suites/:id/run" />
      <DocCurl cmd="POST /api/repos/:id/eval-gate" />
      <DocCurl cmd="POST /api/eval/calibrate" />
      <DocCurl cmd="GET  /api/human-review" />
      <DocCurl cmd="POST /api/human-review/:id/decide" />

      <h2>CI gate</h2>
      <p>Any external CI can hit the gate without an existing session:</p>
      <DocCurl
        cmd={`curl -X POST http://127.0.0.1:8080/api/repos/$REPO/eval-gate \\
  -H 'authorization: Bearer $PROMPTSHEON_API_KEY' \\
  -d '{ "trials":[ { "caseId":"x", "output":"hello", "finalState":{} } ] }'`}
      />
      <p>Returns <code>{`{ ok, score, regressions, suites }`}</code>; CI fails the build when <code>ok=false</code>.</p>

      <DocNext href="/docs/releases" label="Release workflow" />
    </DocPage>
  );
}
