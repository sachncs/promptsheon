package schedule

import (
	"github.com/sachncs/promptsheon/backend/errs"
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
		// Parser returns either errs.ErrorScheduleInvalidCron or a wrapped
		// errs.ErrorScheduleInvalidCron; both are acceptable. Anything else
		// indicates a parser bug.
		if err != nil && !errors.Is(err, errs.ErrorScheduleInvalidCron) {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
