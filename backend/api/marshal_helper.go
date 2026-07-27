package api

import (
	"encoding/json"
	"fmt"
)

// marshalNoArgs returns the JSON encoding of v or an error.
// Library code does not panic on marshalling failures; callers
// surface the error to the HTTP handler so the route returns 500
// rather than crashing the daemon.
func marshalNoArgs(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("api: marshal request: %w", err)
	}
	return b, nil
}
