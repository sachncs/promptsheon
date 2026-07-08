package capability

import "time"

// EventType identifies the kind of domain event.
type EventType string

const (
	// EventCapabilityCreated is emitted when a capability is created.
	EventCapabilityCreated EventType = "capability_created"
	// EventCapabilityUpdated is emitted when a capability is updated.
	EventCapabilityUpdated EventType = "capability_updated"
	// EventCapabilityArchived is emitted when a capability is archived.
	EventCapabilityArchived EventType = "capability_archived"
	// EventVersionCreated is emitted when a version is created.
	EventVersionCreated EventType = "version_created"
	// EventVersionPromoted is emitted when a version is promoted.
	EventVersionPromoted EventType = "version_promoted"
	// EventEvaluationCompleted is emitted when an evaluation completes.
	EventEvaluationCompleted EventType = "evaluation_completed"
	// EventEvaluationThresholdsMet is emitted when evaluation thresholds are met.
	EventEvaluationThresholdsMet EventType = "evaluation_thresholds_met"
	// EventDeploymentStarted is emitted when a deployment starts.
	EventDeploymentStarted EventType = "deployment_started"
	// EventDeploymentSucceeded is emitted when a deployment succeeds.
	EventDeploymentSucceeded EventType = "deployment_succeeded"
	// EventDeploymentFailed is emitted when a deployment fails.
	EventDeploymentFailed EventType = "deployment_failed"
	// EventDeploymentRolledBack is emitted when a deployment is rolled back.
	EventDeploymentRolledBack EventType = "deployment_rolled_back"
	// EventExecutionFinished is emitted when an execution finishes.
	EventExecutionFinished EventType = "execution_finished"
	// EventObservationGenerated is emitted when an observation is generated.
	EventObservationGenerated EventType = "observation_generated"
	// EventRecommendationGenerated is emitted when a recommendation is generated.
	EventRecommendationGenerated EventType = "recommendation_generated"
	// EventRegressionDetected is emitted when a regression is detected.
	EventRegressionDetected EventType = "regression_detected"
	// EventRollbackPerformed is emitted when a rollback is performed.
	EventRollbackPerformed EventType = "rollback_performed"
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
