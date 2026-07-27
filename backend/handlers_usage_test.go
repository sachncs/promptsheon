package backend

import (
	"testing"
	"strconv"
)

func TestUsageTracker_RecordAndGet(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("p1", "Prompt 1", 100, 50.0)
	ut.RecordCapabilityUsage("p1", "Prompt 1", 200, 150.0)

	top := ut.GetTopCapabilities(10)
	if len(top) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(top))
	}
	if top[0].Count != 2 {
		t.Errorf("expected count 2, got %d", top[0].Count)
	}
	if top[0].AvgTokens != 150 {
		t.Errorf("expected avg tokens 150, got %f", top[0].AvgTokens)
	}
}


func TestUsageTracker_RecordAgent(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("a1", "Agent 1", 100, 50.0)
	ut.RecordCapabilityUsage("a1", "Agent 1", 100, 50.0)
	ut.RecordCapabilityUsage("a2", "Agent 2", 100, 50.0)

	top := ut.GetTopCapabilities(10)
	if len(top) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(top))
	}
	if top[0].ID != "a1" || top[0].Count != 2 {
		t.Errorf("expected a1 with count 2, got %s with count %d", top[0].ID, top[0].Count)
	}
}


func TestUsageTracker_GetEmpty(t *testing.T) {
	ut := NewUsageTracker()
	if prompts := ut.GetTopCapabilities(10); len(prompts) != 0 {
		t.Errorf("expected empty, got %d", len(prompts))
	}
	if agents := ut.GetTopCapabilities(10); len(agents) != 0 {
		t.Errorf("expected empty, got %d", len(agents))
	}
}


func TestUsageTracker_GetTopCapabilitiesLimit(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("p1", "P1", 10, 1)
	ut.RecordCapabilityUsage("p2", "P2", 20, 2)
	ut.RecordCapabilityUsage("p3", "P3", 30, 3)
	top := ut.GetTopCapabilities(1)
	if len(top) != 1 {
		t.Errorf("expected 1 result, got %d", len(top))
	}
}


func TestUsageTracker_GetTopCapabilitiesMore(t *testing.T) {
	ut := NewUsageTracker()
	ut.RecordCapabilityUsage("a1", "A1", 100, 50.0)
	ut.RecordCapabilityUsage("a2", "A2", 100, 50.0)
	ut.RecordCapabilityUsage("a3", "A3", 100, 50.0)
	top := ut.GetTopCapabilities(2)
	if len(top) != 2 {
		t.Errorf("expected 2 results, got %d", len(top))
	}
}


func TestUsageTracker_NewUsageTracker(t *testing.T) {
	ut := NewUsageTracker()
	if ut == nil {
		t.Fatal("expected non-nil tracker")
	}
	if ut.entries == nil {
		t.Error("expected initialized map")
	}
	if ut.order == nil {
		t.Error("expected initialized lru list")
	}
}


func TestUsageTracker_LRUEvicts(t *testing.T) {
	// PERF-5: oldest entry is evicted when the cap is hit.
	// We can't easily test with the production cap (16384)
	// here, so we directly exercise the list behaviour.
	ut := NewUsageTracker()
	for i := 0; i < maxUsageEntries; i++ {
		ut.RecordCapabilityUsage(strconv.Itoa(i), "c", 0, 0)
	}
	if ut.order.Len() != maxUsageEntries {
		t.Fatalf("expected full LRU, got %d", ut.order.Len())
	}
	ut.RecordCapabilityUsage("new", "new", 0, 0)
	if ut.order.Len() != maxUsageEntries {
		t.Fatalf("expected LRU cap, got %d", ut.order.Len())
	}
	if _, ok := ut.entries["0"]; ok {
		t.Error("expected entry 0 to be evicted")
	}
	if _, ok := ut.entries["new"]; !ok {
		t.Error("expected entry 'new' to be present")
	}
}

// ---------------------------------------------------------------------------
// storeAuthAdapter Tests
// ---------------------------------------------------------------------------
