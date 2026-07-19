package schedule

import (
	"testing"
	"time"
)

// TestPOSIX_Cron_DOM_DOW_Semantics exercises the four cases the
// POSIX cron DOM/DOW rule must handle. The previous implementation
// always OR'd the two masks, which meant any expression with one
// wildcard fired daily.
func TestPOSIX_Cron_DOM_DOW_Semantics(t *testing.T) {
	monday := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC) // Mon
	tuesday := time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC)
	day1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Wed

	cases := []struct {
		name   string
		expr   string
		from   time.Time
		want   time.Time
	}{
		{"DOM wildcard, DOW=Monday", "0 0 * * 1", tuesday, time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC)},
		{"DOW wildcard, DOM=1", "0 0 1 * *", day1.AddDate(0, 0, 1), time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
		{"both wildcards fire daily", "0 0 * * *", tuesday, tuesday.AddDate(0, 0, 1)},
		{"both restricted, OR (1st OR Mon)", "0 0 1 * 1", day1, monday},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := nextCron(c.expr, c.from)
			if err != nil {
				t.Fatalf("Next: %v", err)
			}
			if !got.Equal(c.want) {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}
