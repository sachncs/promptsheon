package capability

import "context"

// Repository is the consumer-defined persistence interface for the
// Capability aggregate root and its directly-owned value objects
// (Workspace, Project, Version, Execution).
//
// The interface lives in the capability package because that is
// where the consumers (HTTP handlers, workflow engine, the
// optimizer, recommendation loop) are. Storage implementations in
// internal/store satisfy this interface; a Postgres implementation
// added later is a drop-in replacement.
//
// The interface is grouped: methods on each bounded sub-aggregate
// are contiguous. A consumer that only needs Capabilities does not
// depend on the full Repository.
type Repository interface {
	// Workspaces.
	CreateWorkspace(ctx context.Context, w *Workspace) error
	GetWorkspace(ctx context.Context, id string) (*Workspace, error)
	ListWorkspaces(ctx context.Context) ([]*Workspace, error)
	UpdateWorkspace(ctx context.Context, w *Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error

	// Projects.
	CreateProject(ctx context.Context, p *Project) error
	GetProject(ctx context.Context, id string) (*Project, error)
	ListProjects(ctx context.Context, workspaceID string) ([]*Project, error)
	UpdateProject(ctx context.Context, p *Project) error
	DeleteProject(ctx context.Context, id string) error

	// Capabilities.
	CreateCapability(ctx context.Context, c *Capability) error
	GetCapability(ctx context.Context, id string) (*Capability, error)
	ListCapabilities(ctx context.Context, projectID string) ([]*Capability, error)
	UpdateCapability(ctx context.Context, c *Capability) error
	DeleteCapability(ctx context.Context, id string) error

	// Capability Versions.
	CreateVersion(ctx context.Context, v *Version) error
	GetVersion(ctx context.Context, id string) (*Version, error)
	ListVersions(ctx context.Context, capabilityID string) ([]*Version, error)
	GetLatestVersion(ctx context.Context, capabilityID string) (*Version, error)

	// Executions.
	CreateExecution(ctx context.Context, e *Execution) error
	GetExecution(ctx context.Context, id string) (*Execution, error)
	ListExecutions(ctx context.Context, filter ExecutionFilter) ([]*Execution, error)
}

// ExecutionFilter narrows ListExecutions queries.
type ExecutionFilter struct {
	CapabilityVersionID string
	Limit               int
	Offset              int
}
