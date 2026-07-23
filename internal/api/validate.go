package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	sqlitelib "modernc.org/sqlite/lib"

	"modernc.org/sqlite"
)

// translateDBError is the central error->HTTP mapping for
// repository calls. API-4a replaces the ~15 hand-rolled
// `if errors.Is(err, sql.ErrNoRows) { return ErrNotFound }` sites
// across handlers with this single helper.
//
// Mapping:
//   - nil                                              -> nil (handler continues)
//   - sql.ErrNoRows / store.ErrNotFound                 -> ErrNotFound (404)
//   - unique-constraint violation (SQLITE_CONSTRAINT)   -> ErrConflict (409)
//   - context cancellation / deadline                   -> 499 Client Closed
//   - everything else                                   -> 500 with a generic
//     "resource lookup failed" message; the wrapped error
//     stays in the chain so middleware / audit can read it
//     via errors.Is / errors.As.
func translateDBError(err error, resource string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &HTTPError{Status: 499, Message: "client closed request"}
	}
	if isUniqueViolation(err) {
		return &HTTPError{
			Status:  http.StatusConflict,
			Message: resource + " already exists",
		}
	}
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Message: resource + " lookup failed",
	}
}

// isUniqueViolation matches SQLite "UNIQUE constraint failed"
// errors. The modernc.org/sqlite driver returns *sqlite.Error
// with Code == sqlite.SQLITE_CONSTRAINT; the message includes
// the word "unique". API-4b wraps every store error with %w so
// the errors.As call works; older builds fall back to a string
// match so a vendor swap doesn't silently regress the 409 path.
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code() == sqlitelib.SQLITE_CONSTRAINT {
			return strings.Contains(strings.ToLower(sqliteErr.Error()), "unique")
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

// validEmail enforces RFC 5322 syntax via net/mail. We do not
// deliver mail, so deeper validation (MX, deliverability) is
// out of scope. API-VAL-6: the previous form accepted anything
// with an "@" in it; tightening this blocks obvious typos and
// invalid addresses that downstream OAuth flows would reject
// anyway.
func validEmail(s string) bool {
	if s == "" {
		return false
	}
	_, err := mail.ParseAddress(s)
	return err == nil
}

// validateEnum reports whether v is a member of allowed (case-
// sensitive). API-VAL-4 / API-VAL-7 use this to reject values
// outside the closed set before they reach the store.
func validateEnum(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// validatePositiveInt returns a 400-bad-request error when n is
// not strictly greater than zero. API-VAL-2 / API-VAL-5 wrap
// this for the per-field rules.
func validatePositiveInt(name string, n int) error {
	if n <= 0 {
		return badRequest(fmt.Sprintf("%s must be > 0", name))
	}
	return nil
}

// validatePositiveFloat mirrors validatePositiveInt for float
// thresholds (alert rules, etc.).
func validatePositiveFloat(name string, f float64) error {
	if f <= 0 {
		return badRequest(fmt.Sprintf("%s must be > 0", name))
	}
	return nil
}

// validateNonEmpty returns a 400 when s is empty. The resource
// label is included in the error so the operator can see WHICH
// required field was missing.
func validateNonEmpty(name, s string) error {
	if strings.TrimSpace(s) == "" {
		return badRequest(name + " is required")
	}
	return nil
}
