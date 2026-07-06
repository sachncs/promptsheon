package store

import (
	"context"

	"github.com/sachn-cs/promptsheon/internal/capability"
)

// CapabilityRepository defines persistence for capability-centric domain types.
//
// This is a separate interface from the main Repository to keep aggregate
// boundaries clean. It will replace the monolithic Repository interface
// in a later phase.
type CapabilityRepository interface {
	// Workspaces
	CreateWorkspace(ctx context.Context, w *capability.Workspace) error
	GetWorkspace(ctx context.Context, id string) (*capability.Workspace, error)
	ListWorkspaces(ctx context.Context) ([]*capability.Workspace, error)
	UpdateWorkspace(ctx context.Context, w *capability.Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error

	// Projects
	CreateProject(ctx context.Context, p *capability.Project) error
	GetProject(ctx context.Context, id string) (*capability.Project, error)
	ListProjects(ctx context.Context, workspaceID string) ([]*capability.Project, error)
	UpdateProject(ctx context.Context, p *capability.Project) error
	DeleteProject(ctx context.Context, id string) error

	// Capabilities
	CreateCapability(ctx context.Context, c *capability.Capability) error
	GetCapability(ctx context.Context, id string) (*capability.Capability, error)
	ListCapabilities(ctx context.Context, projectID string) ([]*capability.Capability, error)
	UpdateCapability(ctx context.Context, c *capability.Capability) error
	DeleteCapability(ctx context.Context, id string) error

	// Capability Versions
	CreateVersion(ctx context.Context, v *capability.CapabilityVersion) error
	GetVersion(ctx context.Context, id string) (*capability.CapabilityVersion, error)
	ListVersions(ctx context.Context, capabilityID string) ([]*capability.CapabilityVersion, error)
	GetLatestVersion(ctx context.Context, capabilityID string) (*capability.CapabilityVersion, error)

	// Executions
	CreateExecution(ctx context.Context, e *capability.Execution) error
	GetExecution(ctx context.Context, id string) (*capability.Execution, error)
	ListExecutions(ctx context.Context, filter ExecutionFilter) ([]*capability.Execution, error)
}

// ExecutionFilter defines criteria for listing executions.
type ExecutionFilter struct {
	CapabilityVersionID string
	Limit               int
	Offset              int
}
