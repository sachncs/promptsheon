package capability

import "time"

// DeploymentStatus represents the current state of a deployment.
type DeploymentStatus string

const (
	// DeploymentStatusPending is a pending deployment status.
	DeploymentStatusPending DeploymentStatus = "pending"
	// DeploymentStatusActive is an active deployment status.
	DeploymentStatusActive DeploymentStatus = "active"
	// DeploymentStatusRolledBack is a rolled back deployment status.
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
	// DeploymentStatusFailed is a failed deployment status.
	DeploymentStatusFailed DeploymentStatus = "failed"
)

// DeploymentHealth represents the health of an active deployment.
type DeploymentHealth string

const (
	// DeploymentHealthHealthy is a healthy deployment health.
	DeploymentHealthHealthy DeploymentHealth = "healthy"
	// DeploymentHealthDegraded is a degraded deployment health.
	DeploymentHealthDegraded DeploymentHealth = "degraded"
	// DeploymentHealthUnhealthy is an unhealthy deployment health.
	DeploymentHealthUnhealthy DeploymentHealth = "unhealthy"
)

// Deployment represents an immutable deployment of a capability version
// to a specific environment.
type Deployment struct {
	ID                  string           `json:"id"`
	CapabilityVersionID string           `json:"capability_version_id"`
	Environment         string           `json:"environment"` // "dev", "staging", "prod"
	Status              DeploymentStatus `json:"status"`
	Health              DeploymentHealth `json:"health,omitempty"`
	RollbackTargetID    string           `json:"rollback_target_id,omitempty"`
	DeployedAt          time.Time        `json:"deployed_at"`
	RolledBackAt        *time.Time       `json:"rolled_back_at,omitempty"`
}
