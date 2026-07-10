// Package promptsheon is a deprecation shim for pkg/cas.
//
// The content-addressable store implementation has moved to
// github.com/sachncs/promptsheon/pkg/cas per ADR-0013. The shim
// re-exports every exported identifier so existing callers
// continue to compile. New code MUST import pkg/cas directly. The
// shim will be removed in a follow-on cleanup milestone.
//
// Deprecated: use pkg/cas directly.
package promptsheon

import (
	"context"
	"io"

	"github.com/sachncs/promptsheon/pkg/cas"
)

// Type aliases preserve the historical names exactly. Each alias
// points at the canonical type in the new package.
type (
	ObjectType = cas.ObjectType

	TelemetryKV  = cas.TelemetryKV
	Object       = cas.Object
	TreeEntry    = cas.TreeEntry
	CommitResult = cas.CommitResult
	DiffEntry    = cas.DiffEntry
	DiffResult   = cas.DiffResult
	GraphNode    = cas.GraphNode
	LogEntry     = cas.LogEntry
	MetricDiff   = cas.MetricDiff
	RefDetail    = cas.RefDetail
	RepoStats    = cas.RepoStats
	VerifyResult = cas.VerifyResult
)

// Object-type constants. They are typed constants of the form
// `cas.ObjectType("blob")` and have to be re-declared locally
// because Go does not allow re-exporting constants through a
// type-alias indirection. Values match pkg/cas exactly.
const (
	TypeBlob   ObjectType = cas.TypeBlob
	TypeTree   ObjectType = cas.TypeTree
	TypeCommit ObjectType = cas.TypeCommit
)

// Functions: full re-export. Each entry below aliases the canonical
// implementation in pkg/cas. Variadic signatures are preserved by
// re-declaration when the underlying has a concrete shape.

// Object construction.
var (
	NewBlobObject   = cas.NewBlobObject
	NewTreeObject   = cas.NewTreeObject
	NewCommitObject = cas.NewCommitObject
	ObjectHash      = cas.ObjectHash
	WriteObject     = cas.WriteObject
	ReadObject      = cas.ReadObject
	ObjectExists    = cas.ObjectExists
	ObjectFileSize  = cas.ObjectFileSize
)

// Repo lifecycle and operations.
var (
	Init                 = cas.Init
	IsInitialized        = cas.IsInitialized
	Commit               = cas.Commit
	BuildGraph           = cas.BuildGraph
	GetCurrentCommitHash = cas.GetCurrentCommitHash
	GetStats             = cas.GetStats
	Log                  = cas.Log
	Verify               = cas.Verify
)

// Refs and branches.
var (
	CreateBranch   = cas.CreateBranch
	DeleteBranch   = cas.DeleteBranch
	Checkout       = cas.Checkout
	GetCurrentRef  = cas.GetCurrentRef
	ReadHEAD       = cas.ReadHEAD
	WriteHEAD      = cas.WriteHEAD
	ReadRef        = cas.ReadRef
	WriteRef       = cas.WriteRef
	ListRefs       = cas.ListRefs
	ListRefDetails = cas.ListRefDetails
	HEADRefName    = cas.HEADRefName
	IsHEADDetached = cas.IsHEADDetached
)

// Diff and similarity.
var (
	DiffIntelligence = cas.DiffIntelligence
	FormatDiff       = cas.FormatDiff
	SimHash          = cas.SimHash
	SimilarityScore  = cas.SimilarityScore
)

// Logger hook for diagnostics from the repository layer.
var SetLogger = cas.SetLogger

// Reader/writer are imported here only so future additions can
// keep this file stable; their methods on Repo are reachable
// through the type aliases above.
var (
	_ = context.Background
	_ = io.Discard
)
