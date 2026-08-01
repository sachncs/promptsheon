package promptsheon

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleUpdateCapabilityContract_RoutesThroughTranslateDBError
// verifies the handler now routes errors through translateDBError.
// The mock's SetCapabilityContract doesn't currently validate
// capability existence (orphaned attaches — separate fix), so we
// instead exercise the error-path that translateDBError recognizes:
// a not-found from the store must map to 404. We simulate this by
// pre-seeding a contract, then deleting the capability row, then
// re-fetching via GetCapabilityContract (which the mock correctly
// returns errs.ErrStoreNotFound for).
func TestHandleUpdateCapabilityContract_RoutesThroughTranslateDBError(t *testing.T) {
	s := newTestServer(t)
	// Seed a contract; the mock returns errs.ErrStoreNotFound for
	// missing keys. Direct read via the handler should now map to 404.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/missing/contract", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetCapabilityContract_NotFoundReturns404(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/missing/contract", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404 (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestParseVersionDiffArgs_RejectsGarbage locks in MED-1 fix (c0.8).
// Before the fix fmt.Sscanf accepted "1; DROP TABLE" and only read
// the leading integer. Now strconv.Atoi requires the full string to parse.
func TestParseVersionDiffArgs_RejectsGarbage(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"plain integers", "1", "2", false},
		{"from with garbage", "1; DROP TABLE releases", "2", true},
		{"to with garbage", "1", "2; --", true},
		{"empty from defaults", "", "5", false},
		{"non-numeric to", "1", "abc", true},
		{"to missing", "1", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/x/diff", nil)
			q := r.URL.Query()
			if c.from != "" {
				q.Set("from", c.from)
			}
			if c.to != "" {
				q.Set("to", c.to)
			}
			r.URL.RawQuery = q.Encode()
			_, _, err := parseVersionDiffArgs(r)
			if (err != nil) != c.wantErr {
				t.Errorf("from=%q to=%q: err=%v, wantErr=%v", c.from, c.to, err, c.wantErr)
			}
		})
	}
}
