package capability

// MCPServer defines how tools connect to external services.
//
// MCP (Model Context Protocol) is how the system connects to tools.
// This is a different abstraction from Tool itself: a Tool is what
// the model invokes (e.g. "search_github"), while MCP is the transport
// protocol that connects the tool to the execution runtime.
type MCPServer struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Transport string         `json:"transport"` // "stdio", "sse", "grpc"
	Config    map[string]any `json:"config,omitempty"`
}
