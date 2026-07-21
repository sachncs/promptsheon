package pluginsup

import (
	"testing"

	"github.com/sachncs/promptsheon/internal/pluginmanifest"
	"github.com/sachncs/promptsheon/internal/supervisor"
)

// newTestSupervisor returns a supervisor wired to a no-op logger.
// Tests in this package use it as a constructor.
func newTestSupervisor() *supervisor.Supervisor {
	return supervisor.New(nil, newDiscardLogger())
}

func manifestEntry(t *testing.T, name string) manifest.Entry {
	t.Helper()
	return manifest.Entry{
		Name:     name,
		Version:  "1.0.0",
		Binary:   "/opt/" + name,
		Services: []string{"Provider"},
	}
}

func manifestEntryWithUDS(t *testing.T, name, uds string) manifest.Entry {
	t.Helper()
	e := manifestEntry(t, name)
	e.UDS = uds
	return e
}
