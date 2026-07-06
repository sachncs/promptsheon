package capability

import "time"

// EventType identifies the kind of domain event.
type EventType string

const (
	EventCapabilityCreated       EventType = "capability_created"
	EventCapabilityUpdated       EventType = "capability_updated"
	EventCapabilityArchived      EventType = "capability_archived"
	EventVersionCreated          EventType = "version_created"
	EventVersionPromoted         EventType = "version_promoted"
	EventEvaluationCompleted     EventType = "evaluation_completed"
	EventEvaluationThresholdsMet EventType = "evaluation_thresholds_met"
	EventDeploymentStarted       EventType = "deployment_started"
	EventDeploymentSucceeded     EventType = "deployment_succeeded"
	EventDeploymentFailed        EventType = "deployment_failed"
	EventDeploymentRolledBack    EventType = "deployment_rolled_back"
	EventExecutionFinished       EventType = "execution_finished"
	EventObservationGenerated    EventType = "observation_generated"
	EventRecommendationGenerated EventType = "recommendation_generated"
	EventRegressionDetected      EventType = "regression_detected"
	EventRollbackPerformed       EventType = "rollback_performed"
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
