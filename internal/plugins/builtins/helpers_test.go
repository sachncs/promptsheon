package builtins

import (
	"io"
	"log/slog"

	"github.com/sachncs/promptsheon/internal/supervisor"
)

// supervisorForTest returns a Supervisor wired to a no-op logger.
// Tests in this package use it as a constructor.
func supervisorForTest() *supervisor.Supervisor {
	return supervisor.New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
