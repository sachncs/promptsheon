package store

import "strings"

// splitResource splits the legacy audit_entries.resource string
// ("workspace:abc", "release:r1", "user:u1") into a (kind, id)
// pair for the structural query path added in migration 048a.
// Returns ("", "") for an empty or colon-free input. The colon
// is the documented separator; the existing handlers in
// internal/api/handlers_*.go all produce strings in this shape.
func splitResource(resource string) (string, string) {
	i := strings.IndexByte(resource, ':')
	if i < 0 {
		return "", ""
	}
	return resource[:i], resource[i+1:]
}
