//go:build tests_migration


package store

import (
	"strings"
	"testing"
)

// TestMustUnmarshal_ReturnsErrorOnCorruption locks in CRIT-1 / DEF-13
// fix (c0.10). The previous behaviour silently logged the error and
// left the destination at its zero value. Now the parse error
// propagates so callers can surface the corrupted row.
func TestMustUnmarshal_ReturnsErrorOnCorruption(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty input", "", false},
		{"valid JSON", `{"name":"x"}`, false},
		{"truncated JSON", `{"name":`, true},
		{"wrong type", `["a","b"]`, true},
		{"trailing garbage", `{"name":"x"}garbage`, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v map[string]any
			err := mustUnmarshal([]byte(c.input), &v)
			if (err != nil) != c.wantErr {
				t.Errorf("input=%q: err=%v, wantErr=%v", c.input, err, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "unmarshal") {
				t.Errorf("error message %q should mention 'unmarshal'", err.Error())
			}
		})
	}
}
