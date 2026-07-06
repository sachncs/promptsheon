package capability

// Tool represents an external integration that a model can use during execution.
//
// A Tool is what the model uses. Tools have independent versioning so
// a capability can pin to a specific tool version or float to latest.
// Examples: GitHub, Slack, Jira, SQL, REST, Browser, Filesystem.
type Tool struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Type    string         `json:"type"` // "http", "shell", "database", "filesystem", "api"
	Config  map[string]any `json:"config,omitempty"`
}
