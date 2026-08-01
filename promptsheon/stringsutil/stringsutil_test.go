//go:build tests_migration


package stringsutil

import "testing"

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single", "a", []string{"a"}},
		{"trimmed", "a, b ,c", []string{"a", "b", "c"}},
		{"empty_pieces", "a,,b,", []string{"a", "b"}},
		{"only_separators", ", ,", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitCSV(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("len(got)=%d want %d (got=%v want=%v)", len(got), len(c.want), got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("[%d] got %q want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}
