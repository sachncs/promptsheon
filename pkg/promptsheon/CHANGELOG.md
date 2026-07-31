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
package promptsheon
