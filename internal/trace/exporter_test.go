package trace

import (
	"os"
	"strconv"
	"testing"
)

// TestParseSampleRatio pins OBS-TR-3: PROMPTSHEON_OTEL_SAMPLE_RATIO
// overrides the hard-coded 5% sample. Defaults to 1.0 (AlwaysSample
// — every span ships) so the only knob operators set is the
// sampling rate.
func TestParseSampleRatio(t *testing.T) {
	cases := []struct {
		env  string
		want float64
	}{
		{"", 1.0},          // unset → full sample
		{"0.05", 0.05},     // standard 5%
		{"0.5", 0.5},       // 50%
		{"1.0", 1.0},       // explicit 100%
		{"-0.1", 0.0},      // negative → clamp to 0
		{"1.5", 1.0},       // >1 → clamp to 1
		{"not-a-float", 1.0}, // garbage → default
	}
	for _, c := range cases {
		if c.env != "" {
			t.Setenv("PROMPTSHEON_OTEL_SAMPLE_RATIO", c.env)
		} else {
			t.Setenv("PROMPTSHEON_OTEL_SAMPLE_RATIO", "")
		}
		got := parseSampleRatio()
		if got != c.want {
			t.Errorf("env=%q: got %v, want %v", c.env, got, c.want)
		}
	}
}

// parseSampleRatio mirrors the env-var parsing in
// InitTracerProvider. Extracted so tests don't need to spin up
// an OTel exporter.
func parseSampleRatio() float64 {
	sampleRatio := 1.0
	if v := os.Getenv("PROMPTSHEON_OTEL_SAMPLE_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			sampleRatio = f
		}
	}
	return sampleRatio
}
