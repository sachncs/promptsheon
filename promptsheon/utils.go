// Package promptsheon — kitchen-sink utility code.
//
// This file holds small, stateless helpers that don't deserve
// their own subpackage: ID generation, pagination parsing,
// HTTP response helpers, JSON I/O, error construction. The
// promptsheon package itself owns the *Server type, the request
// router, and the larger middlewares (those stay in their
// dedicated files).
package promptsheon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ── ID generation ───────────────────────────────────────────────────

// generateID produces a collision-resistant identifier.
// (body moved from backend/generateid.go)
func generateID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("api-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("api-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
}

// ── Error construction ──────────────────────────────────────────────

// Errorf formats according to a format specifier and returns
// the error as a value. The %w verb is honoured.
//
// This is the single source of fmt usage in the promptsheon
// package; all callers should use errf.Errorf directly to keep
// the "fmt" dependency contained.

// HTTPError represents an HTTP error with a specific status code.
type HTTPError struct {
	Status  int
	Message string
	Details any
}

func (e *HTTPError) Error() string { return e.Message }

// badRequest, notFound, unauthorized, forbidden construct
// HTTPError values with the appropriate status codes.
func badRequest(msg string) error {
	return &HTTPError{Status: http.StatusBadRequest, Message: msg}
}

func notFound(msg string) error {
	return &HTTPError{Status: http.StatusNotFound, Message: msg}
}

func unauthorized() error {
	return &HTTPError{Status: http.StatusUnauthorized, Message: "authentication required"}
}

func forbidden(msg string) error {
	return &HTTPError{Status: http.StatusForbidden, Message: msg}
}

// ── HTTP transport helpers ──────────────────────────────────────────

// isRequestTLS reports whether the inbound request arrived over
// an encrypted channel.
func isRequestTLS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	if err := JSON(w, status, data); err != nil {
		slog.Error("failed to encode json response", "err", err)
	}
}

// writeError writes a JSON error response, inferring the status
// code from known error types.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Status
	} else if errors.Is(err, ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, ErrBadRequest) {
		status = http.StatusBadRequest
	} else if errors.Is(err, ErrConflict) {
		status = http.StatusConflict
	}
	body := map[string]any{"error": err.Error()}
	if httpErr != nil && httpErr.Details != nil {
		body["details"] = httpErr.Details
	}
	if jerr := JSON(w, status, body); jerr != nil {
		slog.Error("failed to encode error response", "err", jerr)
	}
}

// readJSON decodes r.Body into target.
func readJSON(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}

// httpRequestFromContext returns the *http.Request attached to
// ctx by the WithRequest middleware.
func httpRequestFromContext(ctx context.Context) *http.Request {
	if r, ok := ctx.Value(httpRequestKey{}).(*http.Request); ok {
		return r
	}
	return nil
}

// ── Pagination ───────────────────────────────────────────────────────

// paginationDefaults match the audit handler's defaults so every
// list endpoint exposes the same contract.
const (
	defaultListLimit = 50
	maxListLimit     = 1000
)

// parsePagination reads ?limit and ?offset from the query string.
func parsePagination(r *http.Request) (limit, offset int, err error) {
	limit = defaultListLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return 0, 0, badRequest("invalid limit: must be an integer")
		}
		if n < 1 || n > maxListLimit {
			return 0, 0, badRequest("invalid limit: must be between 1 and 1000")
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return 0, 0, badRequest("invalid offset: must be an integer")
		}
		if n < 0 {
			return 0, 0, badRequest("invalid offset: must be non-negative")
		}
		offset = n
	}
	return limit, offset, nil
}

// applyOffsetLimit trims any slice to [offset, offset+limit).
func applyOffsetLimit[T any](rows []T, offset, limit int) []T {
	if offset >= len(rows) {
		return []T{}
	}
	rows = rows[offset:]
	if limit < len(rows) {
		rows = rows[:limit]
	}
	return rows
}

// writePaginationHeaders sets the RFC 5988 Link header.
func writePaginationHeaders(w http.ResponseWriter, r *http.Request, limit, offset, total, returned int) {
	if total >= 0 {
		base := paginationBaseURL(r)
		var links []string
		if offset > 0 {
			prev := offset - limit
			if prev < 0 {
				prev = 0
			}
			links = append(links, fmt.Sprintf(`<%s>; rel="prev"`, paginationLink(base, limit, prev)))
		}
		links = append(links, fmt.Sprintf(`<%s>; rel="first"`, paginationLink(base, limit, 0)))
		if returned == limit && offset+limit < total {
			next := offset + limit
			links = append(links, fmt.Sprintf(`<%s>; rel="next"`, paginationLink(base, limit, next)))
		}
		last := total - limit
		if last < 0 {
			last = 0
		}
		links = append(links, fmt.Sprintf(`<%s>; rel="last"`, paginationLink(base, limit, last)))
		if len(links) > 0 {
			w.Header().Set("Link", joinLinkRel(links))
		}
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
}

// paginationBaseURL returns the request URL minus pagination
// query parameters.
func paginationBaseURL(r *http.Request) string {
	u := *r.URL
	q := u.Query()
	q.Del("limit")
	q.Del("offset")
	u.RawQuery = q.Encode()
	return u.String()
}

// paginationLink formats a single Link target with limit + offset.
func paginationLink(base string, limit, offset int) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	return u.String()
}

// joinLinkRel joins multiple `<url>; rel="..."` segments.
func joinLinkRel(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// ── Common API errors ────────────────────────────────────────────────

var (
	// ErrNotFound is returned when a requested resource is missing.
	ErrNotFound = errors.New("resource not found")
	// ErrBadRequest is returned for invalid client input.
	ErrBadRequest = errors.New("bad request")
	// ErrConflict is returned when a resource already exists.
	ErrConflict = errors.New("resource already exists")
)
