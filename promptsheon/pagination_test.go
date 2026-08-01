//go:build tests_migration


package promptsheon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPaginationHeaders_FirstLinkAlwaysPresent verifies that the
// RFC 5988 rel="first" link is included in the Link header on
// every page, including page 1 (offset=0). Before the fix the
// first link was gated by `if offset > 0` so it was missing
// on page 1.
func TestPaginationHeaders_FirstLinkAlwaysPresent(t *testing.T) {
	cases := []struct {
		name      string
		offset    int
		limit     int
		total     int
		returned  int
		mustFirst bool
		mustPrev  bool
		mustNext  bool
		mustLast  bool
	}{
		{"page 1, more pages exist", 0, 50, 200, 50, true, false, true, true},
		{"page 1, no more pages", 0, 50, 30, 30, true, false, false, true},
		{"middle page", 50, 50, 200, 50, true, true, true, true},
		{"last page", 150, 50, 200, 50, true, true, false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/things?limit=50&offset="+itoa(c.offset), nil)
			w := httptest.NewRecorder()
			writePaginationHeaders(w, r, c.limit, c.offset, c.total, c.returned)

			link := w.Header().Get("Link")
			hasFirst := strings.Contains(link, `rel="first"`)
			hasPrev := strings.Contains(link, `rel="prev"`)
			hasNext := strings.Contains(link, `rel="next"`)
			hasLast := strings.Contains(link, `rel="last"`)

			if hasFirst != c.mustFirst {
				t.Errorf("rel=\"first\": got %v, want %v (Link: %q)", hasFirst, c.mustFirst, link)
			}
			if hasPrev != c.mustPrev {
				t.Errorf("rel=\"prev\": got %v, want %v (Link: %q)", hasPrev, c.mustPrev, link)
			}
			if hasNext != c.mustNext {
				t.Errorf("rel=\"next\": got %v, want %v (Link: %q)", hasNext, c.mustNext, link)
			}
			if hasLast != c.mustLast {
				t.Errorf("rel=\"last\": got %v, want %v (Link: %q)", hasLast, c.mustLast, link)
			}

			if got := w.Header().Get("X-Total-Count"); got != itoa(c.total) {
				t.Errorf("X-Total-Count = %q, want %q", got, itoa(c.total))
			}
		})
	}
}

func itoa(i int) string { return strings.TrimSpace(formatInt(i)) }

// formatInt avoids pulling strconv into this small test file's imports.
func formatInt(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
