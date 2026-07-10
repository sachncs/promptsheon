package pluginsup

import (
	"io"
	"log/slog"
)

// newDiscardLogger is implemented here so the helpers_test.go
// can stay import-light.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
