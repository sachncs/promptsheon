package builtins

import (
	"io"
	"log/slog"

)

// supervisorForTest returns a Supervisor wired to a no-op logger.
// Tests in this package use it as a constructor.
func supervisorForTest() *backend.Supervisor {
	return backend.New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
