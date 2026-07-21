package harness_test

import (
	"strings"
	"testing"

	"github.com/sachncs/promptsheon/internal/harness"
)

func TestValidScorers(t *testing.T) {
	good := []harness.Scorer{
		harness.ScorerExactMatch,
		harness.ScorerContains,
		harness.ScorerRegex,
		harness.ScorerJSONSchema,
	}
	for _, s := range good {
		if !harness.ValidScorers(s) {
			t.Fatalf("expected %q to be valid", s)
		}
	}
	if harness.ValidScorers("not_a_scorer") {
		t.Fatal("expected unknown scorer to be invalid")
	}
}

func TestPreconditionValidate(t *testing.T) {
	cases := []struct {
		name   string
		pre    harness.Precondition
		errMsg string
	}{
		{
			name: "happy path",
			pre: harness.Precondition{
				CapabilityID: "c1", Name: "go-test", Command: "go test ./...", TimeoutSec: 60,
			},
		},
		{
			name:   "missing capability",
			pre:    harness.Precondition{Name: "n", Command: "c", TimeoutSec: 1},
			errMsg: "capability_id is required",
		},
		{
			name:   "missing name",
			pre:    harness.Precondition{CapabilityID: "c", Command: "c", TimeoutSec: 1},
			errMsg: "name is required",
		},
		{
			name:   "missing command",
			pre:    harness.Precondition{CapabilityID: "c", Name: "n", TimeoutSec: 1},
			errMsg: "command is required",
		},
		{
			name: "zero timeout is allowed (means default)",
			pre:  harness.Precondition{CapabilityID: "c", Name: "n", Command: "c", TimeoutSec: 0},
		},
		{
			name:   "negative timeout rejected",
			pre:    harness.Precondition{CapabilityID: "c", Name: "n", Command: "c", TimeoutSec: -1},
			errMsg: "timeout_sec must be non-negative",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pre.Validate()
			if tc.errMsg == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tc.errMsg)
			}
			if !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errMsg)
			}
		})
	}
}
