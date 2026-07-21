package capability

import "time"

// EventType identifies the kind of domain event.
//
// The capability event taxonomy was significantly pruned in
// the DEAD-1 sweep: only the two EventTypes that production code
// actually emits survive. Anything else that was previously
// declared here (capability_created, version_promoted, etc.)
// was speculative scaffolding; reintroduce as needed when
// production code requires them.
type EventType string

const (
	// EventExecutionFinished is emitted when an execution finishes.
	EventExecutionFinished EventType = "execution_finished"
	// EventRecommendationGenerated is emitted when a recommendation is generated.
	EventRecommendationGenerated EventType = "recommendation_generated"
	// EventPluginLifecycle is emitted by the supervisor when a
	// plugin transitions state (started, stopped, crashed, ...).
	// Data carries the plugin name, the lifecycle kind, and any
	// error. Subscribers that care about plugin health filter on
	// this type rather than parsing a generic event payload.
	EventPluginLifecycle EventType = "plugin_lifecycle"
)

// Event represents a domain event — something notable that happened.
//
// Everything produces events. Events are the foundation for audit,
// observability, webhooks, and the feedback loop that drives
// continuous improvement across capability versions.
type Event struct {
	ID            string         `json:"id"`
	Type          EventType      `json:"type"`
	AggregateID   string         `json:"aggregate_id"`
	AggregateType string         `json:"aggregate_type"` // "capability", "version", "deployment", etc.
	Data          map[string]any `json:"data,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

// Note: Deployment-status events were removed in commit e17bff8
// because the Deployment aggregate was merged into the Release
// aggregate. The Release lifecycle (Pending -> Approved -> Active
// -> Superseded/RolledBack) carries the same semantics.
