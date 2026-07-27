package observation

import (
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/executor"
)

// TestAggregatorBoundedWindow verifies that adding more than
// maxRecordsPerWindow records does not grow the in-memory
// window. The previous implementation appended unbounded and
// leaked memory.
func TestAggregatorBoundedWindow(t *testing.T) {
	a := NewAggregator(nil)
	now := time.Now()
	for i := 0; i < maxRecordsPerWindow+1000; i++ {
		a.Add(executor.ExecutionRecord{
			CapabilityID: "c1",
			ReleaseID:    "r1",
			Environment:  "prod",
			ManifestHash: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
		})
	}
	a.mu.Lock()
	bucket := a.records[windowKey{CapabilityID: "c1", VersionID: "r1", Environment: "prod"}]
	a.mu.Unlock()
	bucket.mu.Lock()
	n := len(bucket.records)
	bucket.mu.Unlock()
	if n != maxRecordsPerWindow {
		t.Fatalf("window size = %d want %d", n, maxRecordsPerWindow)
	}
}
