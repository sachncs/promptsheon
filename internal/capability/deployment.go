package capability

import "time"

// DeploymentStatus represents the current state of a deployment.
type DeploymentStatus string

const (
	DeploymentStatusPending   DeploymentStatus = "pending"
	DeploymentStatusActive    DeploymentStatus = "active"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
	DeploymentStatusFailed    DeploymentStatus = "failed"
)

// DeploymentHealth represents the health of an active deployment.
type DeploymentHealth string

const (
	DeploymentHealthHealthy   DeploymentHealth = "healthy"
	DeploymentHealthDegraded  DeploymentHealth = "degraded"
	DeploymentHealthUnhealthy DeploymentHealth = "unhealthy"
)

// Deployment represents an immutable deployment of a capability version
// to a specific environment.
type Deployment struct {
	ID                   string            `json:"id"`
	CapabilityVersionID  string            `json:"capability_version_id"`
	Environment          string            `json:"environment"` // "dev", "staging", "prod"
	Status               DeploymentStatus  `json:"status"`
	Health               DeploymentHealth  `json:"health,omitempty"`
	RollbackTargetID     string            `json:"rollback_target_id,omitempty"`
	DeployedAt           time.Time         `json:"deployed_at"`
	RolledBackAt         *time.Time        `json:"rolled_back_at,omitempty"`
}
