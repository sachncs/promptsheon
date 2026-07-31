//go:build promptsheon

package promptsheon

import (
	"errors"

	"github.com/sachncs/promptsheon/sdk"
)

// APIError is the typed error returned by every SDK method when
// the server returns a non-2xx status. Use errors.As to inspect:
//
//	if apiErr := errors.As(err, &promptsheon.APIError{}); apiErr != nil {
//	    log.Printf("server returned %d: %s", apiErr.StatusCode, apiErr.Message)
//	}
type APIError = sdk.APIError

// isAPIError is the implementation backing the deprecated
// IsAPIError helper. Prefer errors.As directly.
func isAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}