package schedule

import (
	"errors"
	"testing"
	"time"
)

func FuzzNextFireAt(f *testing.F) {
	f.Add("*/5 * * * *")
	f.Add("0 0 * * *")
	f.Add("0,30 * * * *")
	f.Add("")

	f.Fuzz(func(t *testing.T, expr string) {
		from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := nextCron(expr, from)
		// Parser returns either ErrInvalidCron or a wrapped
		// ErrInvalidCron; both are acceptable. Anything else
		// indicates a parser bug.
		if err != nil && !errors.Is(err, ErrInvalidCron) {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
