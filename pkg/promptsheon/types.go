//go:build promptsheon

package promptsheon

import (
	"github.com/sachncs/promptsheon/sdk"
)

// Re-exported types from the canonical SDK. The aliases give
// consumers a stable import path: github.com/sachncs/promptsheon/pkg/promptsheon
// instead of the internal github.com/sachncs/promptsheon/sdk.
//
// Only types that actually exist in the SDK are re-exported;
// adding a name here that the SDK doesn't expose is a build
// error caught by `make check-public`.

type (
	Workspace    = sdk.Workspace
	Project      = sdk.Project
	Capability   = sdk.Capability
	ArtifactRef  = sdk.ArtifactRef
	Manifest     = sdk.Manifest
	Version      = sdk.Version
	Release      = sdk.Release
	Vote         = sdk.Vote
	Approval     = sdk.Approval
	Execution    = sdk.Execution
	Dataset      = sdk.Dataset
	DatasetCase  = sdk.DatasetCase
	Precondition = sdk.Precondition
	EvalRun      = sdk.EvalRun
	EvalResult   = sdk.EvalResult
	APIKey       = sdk.APIKey
	ProviderInfo = sdk.ProviderInfo
	HealthResponse = sdk.HealthResponse

	// Request types
	CreateWorkspaceRequest   = sdk.CreateWorkspaceRequest
	CreateCapabilityRequest   = sdk.CreateCapabilityRequest
	AddVersionRequest         = sdk.AddVersionRequest
	CreateReleaseRequest      = sdk.CreateReleaseRequest
	VoteRequest               = sdk.VoteRequest
	InvokeRequest             = sdk.InvokeRequest
	CreateDatasetRequest      = sdk.CreateDatasetRequest
	CreatePreconditionRequest = sdk.CreatePreconditionRequest
	UpdatePreconditionRequest = sdk.UpdatePreconditionRequest
	RunEvalRequest            = sdk.RunEvalRequest
	CreateAPIKeyRequest       = sdk.CreateAPIKeyRequest
)