//go:build promptsheon

// CHANGELOG for the public SDK. This package is the only public
// Go surface; the legacy github.com/sachncs/promptsheon/sdk
// import path was removed in v1.0.0.
//
// # v1.0.0 (L-1 / breaking change)
//
// The legacy github.com/sachncs/promptsheon/sdk package was
// removed. Consumers must update their import path:
//
//	import "github.com/sachncs/promptsheon/sdk"
// becomes
//	import "github.com/sachncs/promptsheon/pkg/promptsheon"
//
// The change is mechanical: every exported symbol in sdk/ is
// present in pkg/promptsheon with the same name and behaviour.
//
// Why remove sdk/ rather than deprecate it? Per PLAN-49 / L-1
// (the user requested "no backward shims, breaking changes
// welcomed"): the legacy path made every internal type public,
// which prevented refactors. The //go:build promptsheon fence
// in pkg/promptsheon is the only public surface; sdk/ would
// have been a backdoor.
//
// # v1.0.0 - Python and TypeScript SDK directories removed
//
// The sdk/python/ and sdk/typescript/ directories contained
// only a copy of the OpenAPI spec and no actual client code.
// They were misleading: the README, ROADMAP, and CI all
// advertised Python + TypeScript SDKs that did not exist as
// runnable code. They are deleted; the sdk-python and
// sdk-typescript CI jobs and the make sdk / make sdk-check
// Makefile targets are removed at the same time.
//
// This package remains the only SDK surface; see
// docs/reference/sdk.md for the Go client documentation.
package promptsheon
