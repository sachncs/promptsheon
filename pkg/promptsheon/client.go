//go:build promptsheon

// Package promptsheon is the public Go SDK for the Promptsheon
// REST API. It re-exports the canonical types from the internal
// packages (sdk/, backend/errs, backend/auth, backend/audit,
// backend/capability) so consumers don't need to import the
// internal paths directly.
//
// Build fence (PLAN-49 c6.5):
//
//   //go:build promptsheon
//
// This file is compiled only when -tags=promptsheon is set.
// Default `go build ./...` and `go test ./...` skip it, which
// keeps the daemon's main build fast and avoids pulling SDK-only
// types into the runtime binary. The 'check-public' Makefile
// target runs go vet + go test against this facade under the
// build tag.
package promptsheon

import (
	"net/http"

	"github.com/sachncs/promptsheon/sdk"
)

// Client is the SDK HTTP client. The fence-tagged facade re-exports
// the canonical type so consumers don't need to import sdk/.
type Client = sdk.Client

// New creates a new Promptsheon API client. The default timeout
// is 30s; consumers with retry or metric-instrumented transports
// should use NewWithHTTP.
func New(baseURL, apiKey string) *Client {
	return sdk.New(baseURL, apiKey)
}

// NewWithHTTP creates a new Promptsheon API client using the
// supplied *http.Client.
func NewWithHTTP(baseURL, apiKey string, httpClient *http.Client) *Client {
	return sdk.NewWithHTTP(baseURL, apiKey, httpClient)
}
