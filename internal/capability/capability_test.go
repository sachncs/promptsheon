package capability

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCapabilityDefaults(t *testing.T) {
	now := time.Now()
	c := Capability{
		ID:          "cap-1",
		ProjectID:   "proj-1",
		Name:        "Summarize Invoice",
		Description: "Extract and summarize key fields from invoice PDFs",
		Owner:       "alice",
		Tags:        []string{"invoices", "finance"},
		State:       CapabilityStateDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if c.ID != "cap-1" {
		t.Errorf("expected cap-1, got %s", c.ID)
	}
	if c.State != CapabilityStateDraft {
		t.Errorf("expected draft, got %s", c.State)
	}
	if len(c.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(c.Tags))
	}
}

func TestCapabilityJSONRoundTrip(t *testing.T) {
	now := time.Now()
	c := Capability{
		ID:          "cap-2",
		ProjectID:   "proj-1",
		Name:        "Review PR",
		Owner:       "bob",
		State:       CapabilityStateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Capability
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != c.ID {
		t.Errorf("ID: got %s, want %s", got.ID, c.ID)
	}
	if got.Name != c.Name {
		t.Errorf("Name: got %s, want %s", got.Name, c.Name)
	}
	if got.State != c.State {
		t.Errorf("State: got %s, want %s", got.State, c.State)
	}
}

func TestCapabilityVersionImmutable(t *testing.T) {
	v := CapabilityVersion{
		ID:           "ver-1",
		CapabilityID: "cap-1",
		Version:      1,
		Prompt: Prompt{
			Instructions: "Summarize this invoice",
		},
		RuntimePolicy: RuntimePolicy{
			Temperature: 0.2,
			MaxTokens:   1000,
		},
	}

	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
	if v.Prompt.Instructions != "Summarize this invoice" {
		t.Errorf("unexpected prompt instructions")
	}
	if v.RuntimePolicy.Temperature != 0.2 {
		t.Errorf("unexpected temperature")
	}
}

func TestCapabilityVersionJSONRoundTrip(t *testing.T) {
	v := CapabilityVersion{
		ID:           "ver-2",
		CapabilityID: "cap-2",
		Version:      2,
		Prompt: Prompt{
			Role:         "assistant",
			Instructions: "Help the user",
			Examples: []PromptExample{
				{Input: "hello", Output: "hi there"},
			},
			Variables: []PromptVariable{
				{Name: "name", Type: "string", Required: true},
			},
		},
		ModelPolicy: ModelPolicy{
			Requirements: ModelRequirements{
				NeedsReasoning: true,
				MaxLatencyMs:   2000,
			},
			SelectionStrategy: SelectionStrategyQualityOptimized,
		},
		RuntimePolicy: RuntimePolicy{
			Temperature: 0.7,
			MaxTokens:   2048,
		},
	}

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got CapabilityVersion
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != v.ID {
		t.Errorf("ID mismatch")
	}
	if got.Prompt.Instructions != v.Prompt.Instructions {
		t.Errorf("Prompt.Instructions mismatch")
	}
	if len(got.Prompt.Examples) != 1 {
		t.Errorf("expected 1 example, got %d", len(got.Prompt.Examples))
	}
	if got.ModelPolicy.SelectionStrategy != SelectionStrategyQualityOptimized {
		t.Errorf("SelectionStrategy mismatch")
	}
	if got.RuntimePolicy.Temperature != 0.7 {
		t.Errorf("Temperature mismatch")
	}
}

func TestPromptAllFields(t *testing.T) {
	p := Prompt{
		Role:         "assistant",
		Instructions: "Extract data from the document",
		Examples: []PromptExample{
			{Input: "doc1.pdf", Output: "total: $100"},
			{Input: "doc2.pdf", Output: "total: $200"},
		},
		Variables: []PromptVariable{
			{Name: "document_id", Type: "string", Required: true, Description: "The document ID"},
			{Name: "lang", Type: "string", Required: false, Default: "en"},
		},
		Template: "document_id: {{.document_id}}",
		LocaleVariants: map[string]string{
			"fr": "document_id: {{.document_id}}",
			"de": "dokument_id: {{.document_id}}",
		},
	}

	if len(p.Examples) != 2 {
		t.Errorf("expected 2 examples, got %d", len(p.Examples))
	}
	if len(p.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(p.Variables))
	}
	if len(p.LocaleVariants) != 2 {
		t.Errorf("expected 2 locale variants, got %d", len(p.LocaleVariants))
	}
	if !p.Variables[0].Required {
		t.Errorf("first variable should be required")
	}
	if p.Variables[1].Default != "en" {
		t.Errorf("expected default 'en', got %s", p.Variables[1].Default)
	}
}

func TestModelPolicySelectionStrategies(t *testing.T) {
	strategies := []SelectionStrategy{
		SelectionStrategyCostOptimized,
		SelectionStrategyLatencyOptimized,
		SelectionStrategyQualityOptimized,
		SelectionStrategyManual,
	}

	for _, s := range strategies {
		mp := ModelPolicy{
			Requirements: ModelRequirements{
				NeedsJSON:    true,
				NeedsToolUse: true,
			},
			SelectionStrategy: s,
		}

		data, err := json.Marshal(mp)
		if err != nil {
			t.Fatalf("marshal strategy %s: %v", s, err)
		}

		var got ModelPolicy
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal strategy %s: %v", s, err)
		}

		if got.SelectionStrategy != s {
			t.Errorf("strategy mismatch: got %s, want %s", got.SelectionStrategy, s)
		}
	}
}

func TestContextContractFields(t *testing.T) {
	cc := ContextContract{
		RequiredContext: []ContextRef{
			{Key: "user_input", Source: "session"},
			{Key: "document", Source: "knowledge"},
		},
		OptionalContext: []ContextRef{
			{Key: "conversation_history", Source: "memory"},
		},
		ForbiddenContext:   []string{"password", "secret"},
		MaximumSize:        4096,
		CompressionStrategy: "summary",
		RetrievalStrategy:   "hybrid",
	}

	if len(cc.RequiredContext) != 2 {
		t.Errorf("expected 2 required context refs")
	}
	if len(cc.ForbiddenContext) != 2 {
		t.Errorf("expected 2 forbidden patterns")
	}
	if cc.MaximumSize != 4096 {
		t.Errorf("expected max size 4096")
	}
	if cc.RetrievalStrategy != "hybrid" {
		t.Errorf("expected hybrid retrieval")
	}
}

func TestKnowledgeSourceTypes(t *testing.T) {
	sources := []string{"documents", "index", "rag", "graph", "sql", "files"}
	for _, st := range sources {
		ks := KnowledgeSource{
			ID:   "ks-1",
			Name: "test",
			Type: st,
		}
		if ks.Type != st {
			t.Errorf("type mismatch: %s", st)
		}
	}
}

func TestMemoryConfig(t *testing.T) {
	mc := MemoryConfig{
		SessionMemory:    true,
		ConversationMemory: true,
		WorkingMemory:    false,
		LongTermMemory:   false,
		SharedMemory:     false,
		MaxSessionTokens: 8192,
	}

	if !mc.SessionMemory {
		t.Errorf("expected session memory enabled")
	}
	if mc.WorkingMemory {
		t.Errorf("expected working memory disabled")
	}
	if mc.MaxSessionTokens != 8192 {
		t.Errorf("expected 8192 max session tokens")
	}
}

func TestGuardrailPhases(t *testing.T) {
	g := Guardrail{
		ID:      "gr-1",
		Name:    "pii-detection",
		Phase:   GuardrailPhasePre,
		Version: "1.0",
		Config: map[string]any{
			"patterns": []string{"ssn", "credit_card"},
		},
		Threshold: 0.95,
		Severity:  "high",
	}

	if g.Phase != GuardrailPhasePre {
		t.Errorf("expected pre phase")
	}
	if g.Severity != "high" {
		t.Errorf("expected high severity")
	}

	// Test all three phases
	phases := []GuardrailPhase{GuardrailPhasePre, GuardrailPhaseRuntime, GuardrailPhasePost}
	for _, p := range phases {
		g.Phase = p
		data, _ := json.Marshal(g)
		var got Guardrail
		json.Unmarshal(data, &got)
		if got.Phase != p {
			t.Errorf("phase round-trip failed for %s", p)
		}
	}
}

func TestTool(t *testing.T) {
	tool := Tool{
		ID:      "t-1",
		Name:    "search-github",
		Version: "2.1.0",
		Type:    "http",
		Config: map[string]any{
			"url": "https://api.github.com",
		},
	}

	if tool.Type != "http" {
		t.Errorf("expected http type")
	}
	if tool.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0")
	}
}

func TestMCPServer(t *testing.T) {
	mcp := MCPServer{
		ID:        "mcp-1",
		Name:      "github-mcp",
		Transport: "sse",
		Config: map[string]any{
			"endpoint": "https://mcp.github.com",
		},
	}

	if mcp.Transport != "sse" {
		t.Errorf("expected sse transport")
	}
}

func TestRuntimePolicy(t *testing.T) {
	rp := RuntimePolicy{
		Retries:         3,
		TimeoutMs:       30000,
		Streaming:       true,
		Parallelism:     1,
		Caching:         "semantic",
		Temperature:     0.3,
		MaxTokens:       4096,
		ReasoningBudget: 0,
	}

	if rp.Retries != 3 {
		t.Errorf("expected 3 retries")
	}
	if rp.Caching != "semantic" {
		t.Errorf("expected semantic caching")
	}
	if rp.Temperature != 0.3 {
		t.Errorf("expected temperature 0.3")
	}
}

func TestEvaluationSuite(t *testing.T) {
	es := EvaluationSuite{
		Datasets: []EvalDatasetRef{
			{ID: "ds-1", Name: "invoice-test-set"},
		},
		Metrics: []string{"accuracy", "hallucination", "latency"},
		Thresholds: map[string]float64{
			"accuracy":      0.9,
			"hallucination": 0.05,
		},
		RegressionTests: []string{"reg-1", "reg-2"},
		SecurityTests:   []string{"sec-1"},
	}

	if len(es.Datasets) != 1 {
		t.Errorf("expected 1 dataset")
	}
	if len(es.Metrics) != 3 {
		t.Errorf("expected 3 metrics")
	}
	if es.Thresholds["accuracy"] != 0.9 {
		t.Errorf("expected accuracy threshold 0.9")
	}
}

func TestExecution(t *testing.T) {
	now := time.Now()
	e := Execution{
		ID:                  "exec-1",
		CapabilityVersionID: "ver-1",
		Timestamp:           now,
		Inputs:              map[string]any{"text": "hello"},
		Outputs:             map[string]any{"response": "hi"},
		Model:               "gpt-4",
		Provider:            "openai",
		LatencyMs:           1200,
		CostUSD:             0.015,
		PromptTokens:        500,
		CompletionTokens:    100,
		TotalTokens:         600,
		TraceID:             "trace-abc",
		Environment:         "prod",
	}

	if e.LatencyMs != 1200 {
		t.Errorf("expected 1200ms latency")
	}
	if e.TotalTokens != 600 {
		t.Errorf("expected 600 total tokens")
	}
	if e.Model != "gpt-4" {
		t.Errorf("expected gpt-4")
	}
	if e.Environment != "prod" {
		t.Errorf("expected prod env")
	}
}

func TestObservation(t *testing.T) {
	start := time.Now()
	end := start.Add(1 * time.Hour)

	o := Observation{
		CapabilityVersionID: "ver-1",
		PeriodStart:         start,
		PeriodEnd:           end,
		P95LatencyMs:        850,
		P99LatencyMs:        2200,
		AvgCostUSD:          0.012,
		HallucinationRate:   0.02,
		SuccessRate:         0.98,
		Availability:        0.995,
		ExecutionCount:      15000,
	}

	if o.P95LatencyMs != 850 {
		t.Errorf("expected P95 850ms")
	}
	if o.SuccessRate != 0.98 {
		t.Errorf("expected 98%% success rate")
	}
	if o.ExecutionCount != 15000 {
		t.Errorf("expected 15000 executions")
	}
}

func TestEvaluationResult(t *testing.T) {
	er := EvaluationResult{
		CapabilityVersionID: "ver-1",
		Accuracy:            0.95,
		Precision:           0.93,
		Recall:              0.91,
		Hallucination:       0.03,
		LatencyMs:           750,
		CostUSD:             0.008,
		Schema:              1.0,
		Groundedness:        0.97,
		ThresholdsMet:       true,
	}

	if !er.ThresholdsMet {
		t.Errorf("expected thresholds met")
	}
	if er.Accuracy != 0.95 {
		t.Errorf("expected 95%% accuracy")
	}
	if er.Schema != 1.0 {
		t.Errorf("expected schema score 1.0")
	}
}

func TestRecommendationTypes(t *testing.T) {
	types := []RecommendationType{
		RecommendationSwitchModel,
		RecommendationCompressPrompt,
		RecommendationReduceContext,
		RecommendationEnableCache,
		RecommendationDisableReasoning,
		RecommendationUpgradeMCP,
		RecommendationRemoveTool,
		RecommendationSplitCapability,
		RecommendationAddGuardrail,
		RecommendationTunePolicy,
	}

	for _, rt := range types {
		r := Recommendation{
			ID:                   "rec-1",
			CapabilityVersionID:  "ver-1",
			Type:                 rt,
			Reason:               "test reason",
			Confidence:           0.85,
			Impact:               "medium",
			AutoApplicable:       true,
		}

		if r.Type != rt {
			t.Errorf("recommendation type mismatch: %s", rt)
		}
		if r.Confidence != 0.85 {
			t.Errorf("expected confidence 0.85")
		}
	}
}

func TestDeploymentLifecycle(t *testing.T) {
	now := time.Now()

	d := Deployment{
		ID:                   "dep-1",
		CapabilityVersionID:  "ver-1",
		Environment:          "prod",
		Status:               DeploymentStatusPending,
		DeployedAt:           now,
	}

	if d.Status != DeploymentStatusPending {
		t.Errorf("expected pending status")
	}

	// Transition to active
	d.Status = DeploymentStatusActive
	d.Health = DeploymentHealthHealthy

	if d.Status != DeploymentStatusActive {
		t.Errorf("expected active status")
	}
	if d.Health != DeploymentHealthHealthy {
		t.Errorf("expected healthy")
	}

	// Transition to rolled back
	rb := now.Add(30 * time.Minute)
	d.Status = DeploymentStatusRolledBack
	d.RolledBackAt = &rb

	if d.Status != DeploymentStatusRolledBack {
		t.Errorf("expected rolled back")
	}
	if d.RolledBackAt == nil {
		t.Errorf("expected rolled back timestamp")
	}
}

func TestDeploymentHealthValues(t *testing.T) {
	healthValues := []DeploymentHealth{
		DeploymentHealthHealthy,
		DeploymentHealthDegraded,
		DeploymentHealthUnhealthy,
	}

	for _, h := range healthValues {
		d := Deployment{
			Status: DeploymentStatusActive,
			Health: h,
		}
		if d.Health != h {
			t.Errorf("health round-trip failed for %s", h)
		}
	}
}

func TestEventTypes(t *testing.T) {
	eventTypes := []EventType{
		EventCapabilityCreated,
		EventCapabilityUpdated,
		EventCapabilityArchived,
		EventVersionCreated,
		EventVersionPromoted,
		EventEvaluationCompleted,
		EventEvaluationThresholdsMet,
		EventDeploymentStarted,
		EventDeploymentSucceeded,
		EventDeploymentFailed,
		EventDeploymentRolledBack,
		EventExecutionFinished,
		EventObservationGenerated,
		EventRecommendationGenerated,
		EventRegressionDetected,
		EventRollbackPerformed,
	}

	now := time.Now()
	for _, et := range eventTypes {
		e := Event{
			ID:          "evt-1",
			Type:        et,
			AggregateID: "agg-1",
			Timestamp:   now,
		}

		if e.Type != et {
			t.Errorf("event type mismatch: %s", et)
		}
	}
}

func TestEventCorrelation(t *testing.T) {
	e1 := Event{
		ID:            "evt-1",
		Type:          EventExecutionFinished,
		AggregateID:   "ver-1",
		AggregateType: "version",
		Data: map[string]any{
			"execution_id": "exec-1",
			"latency_ms":   1200,
		},
		CorrelationID: "corr-1",
	}

	if e1.CorrelationID != "corr-1" {
		t.Errorf("correlation ID mismatch")
	}
	if e1.Data["execution_id"] != "exec-1" {
		t.Errorf("data mismatch")
	}
}

func TestWorkspace(t *testing.T) {
	now := time.Now()
	w := Workspace{
		ID:           "ws-1",
		Name:         "Acme Corp",
		Organization: "Acme Corporation",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if w.Name != "Acme Corp" {
		t.Errorf("expected Acme Corp")
	}
}

func TestProject(t *testing.T) {
	now := time.Now()
	p := Project{
		ID:          "proj-1",
		WorkspaceID: "ws-1",
		Name:        "Customer Support",
		Description: "AI-powered customer support agent",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if p.WorkspaceID != "ws-1" {
		t.Errorf("workspace ID mismatch")
	}
	if p.Name != "Customer Support" {
		t.Errorf("expected Customer Support")
	}
}

func TestKnowledgeSourceWithEmbedding(t *testing.T) {
	ks := KnowledgeSource{
		ID:             "ks-2",
		Name:           "product-docs",
		Type:           "rag",
		Version:        "2024-01-15",
		EmbeddingModel: "text-embedding-3-small",
		Config: map[string]any{
			"chunk_size":  512,
			"overlap":     50,
			"index_type":  "hnsw",
		},
	}

	if ks.EmbeddingModel != "text-embedding-3-small" {
		t.Errorf("embedding model mismatch")
	}
	raw, ok := ks.Config["chunk_size"]
	if !ok {
		t.Fatal("chunk_size not found in config")
	}
	chunkSize, ok := raw.(int)
	if !ok || chunkSize != 512 {
		t.Errorf("expected chunk_size 512, got %T=%v", raw, raw)
	}
}

func TestCapabilityStateValues(t *testing.T) {
	states := []CapabilityState{
		CapabilityStateDraft,
		CapabilityStateActive,
		CapabilityStateDeprecated,
		CapabilityStateArchived,
	}

	for _, s := range states {
		c := Capability{State: s}
		if c.State != s {
			t.Errorf("state round-trip failed for %s", s)
		}
	}
}

func TestContextContractZeroValues(t *testing.T) {
	cc := ContextContract{}
	if cc.MaximumSize != 0 {
		t.Errorf("expected zero max size")
	}
	if cc.RetrievalStrategy != "" {
		t.Errorf("expected empty retrieval strategy")
	}
}

func TestRuntimePolicyDefaults(t *testing.T) {
	rp := RuntimePolicy{}
	if rp.Retries != 0 {
		t.Errorf("expected 0 retries")
	}
	if rp.Streaming != false {
		t.Errorf("expected streaming false")
	}
	if rp.Temperature != 0 {
		t.Errorf("expected temperature 0")
	}
}
