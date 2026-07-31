//go:build promptsheon

package promptsheon

import (
	"errors"

	"github.com/sachncs/promptsheon/backend/errs"
)

// Re-exported sentinel errors. All sentinels follow the
// idiomatic Err* naming (PLAN-49 c2.8). Use errors.Is to compare.
var (
	ErrNotLeader       = errs.ErrNotLeader
	ErrProviderMissing = errs.ErrProviderMissing
	ErrReleaseNotPending = errs.ErrReleaseNotPending
	ErrRecommendationUnknown = errs.ErrRecommendationUnknown
	ErrStoreNotFound   = errs.ErrStoreNotFound
	ErrStoreConflict   = errs.ErrStoreConflict
	ErrPrecondition    = errs.ErrPrecondition
	ErrSelfVote         = errs.ErrSelfVote
	ErrQuorum           = errs.ErrQuorum
	ErrVaultUnknown     = errs.ErrVaultUnknown
	ErrVaultStopped     = errs.ErrVaultStopped
	ErrApprovalNotFound = errs.ErrApprovalNotFound
	ErrContextExhausted = errs.ErrContextExhausted
	ErrInvalidCron      = errs.ErrInvalidCron
)

// IsAPIError reports whether err is an SDK APIError. Use the
// returned *APIError to inspect status code and message. Prefer
// errors.As directly in new code.
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}