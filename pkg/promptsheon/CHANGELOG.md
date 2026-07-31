//go:build promptsheon

package promptsheon

// CHANGELOG for the public SDK facade. The facade itself is a
// thin re-export layer over github.com/sachncs/promptsheon/sdk
// and the various backend packages; substantive changes happen
// there and propagate here.
//
// # v0.4.0 (PLAN-49 / v1.0.0 release)
//
// Initial fence-tagged facade. Adds:
//   - Client, New, NewWithHTTP (re-exports of sdk.Client + constructors)
//   - APIError (re-export of sdk.APIError)
//   - Workspace, Project, Capability, Version, Release, Execution,
//     Dataset, DatasetCase, Precondition, EvalRun, EvalResult,
//     APIKey, ProviderKey, Alert, AlertRule, NotificationGroup,
//     AuditEntry, and the corresponding Request types
//   - Err* sentinels (re-exports of backend/errs)
//   - Audit key constants (re-exports of backend/audit)
//   - Role + Permission constants (re-exports of backend/auth)
//
// Build: `GOFLAGS=-tags=promptsheon go build ./pkg/promptsheon`
//
// The facade compiles only with the `promptsheon` build tag.
// Default `go build ./...` and `go test ./...` skip it, which
// keeps the daemon's main build fast.
