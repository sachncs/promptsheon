package api

import (
	"net/http"
	"strconv"
)

// paginationDefaults match the audit handler's defaults so every
// list endpoint exposes the same contract.
const (
	defaultListLimit = 50
	maxListLimit     = 1000
)

// parsePagination reads ?limit and ?offset from the query string,
// applying the standard defaults and bounds. A 400 is returned
// when the values are not integers or are out of range.
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
// The store layer returns the full result set and the handler
// applies pagination; this is correct for the current scale
// (hundreds of rows) and avoids the per-collection store
// signature changes that a SQL-side LIMIT would require.
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
