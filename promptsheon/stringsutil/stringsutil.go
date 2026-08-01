// Package stringsutil provides small, allocation-conscious
// string helpers shared across the backend.
//
// The package is intentionally narrow: it is not a replacement
// for the standard library's strings package, only a home for
// the handful of utility functions the backend otherwise
// duplicates per package.
package stringsutil

import "strings"

// SplitCSV splits a comma-separated value into a slice of
// trimmed non-empty strings. Returns nil when the input is
// empty or contains no non-empty entries.
//
// SplitCSV is the canonical CSV splitter for query parameters
// and short, operator-supplied lists. The previous schedule and
// ws implementations differed on whitespace handling and the
// treatment of trailing separators; SplitCSV consolidates the
// behaviour and replaces both.
func SplitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
