package api

import (
	"fmt"
	"net/http"
	"net/url"
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

// writePaginationHeaders sets the RFC 5988 Link header on a
// paginated response. API-3b: the Link header lets clients
// discover the next / previous pages without parsing the body.
//
//	<url?limit=&offset=>; rel="next"   — when a next page may exist
//	<url?limit=&offset=>; rel="prev"   — when a previous page exists
//	<url?limit=&offset=>; rel="first"  — always
//	<url?limit=&offset=>; rel="last"   — when the total is known
//
// `total` is the unpaginated row count; pass -1 when unknown.
// `returned` is the number of rows actually serialised in the
// current page (so the "next" link is omitted when the current
// page was the last one).
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
		if offset > 0 {
			links = append(links, fmt.Sprintf(`<%s>; rel="first"`, paginationLink(base, limit, 0)))
		}
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
// query parameters. We rebuild the URL instead of using
// r.URL.RequestURI() so the link is human-grep-able in logs.
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

// joinLinkRel joins multiple `<url>; rel="..."` segments with
// ", " as RFC 5988 requires.
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
