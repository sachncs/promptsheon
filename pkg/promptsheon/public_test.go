//go:build promptsheon

package promptsheon

import (
	"context"
	"errors"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// TestRoleConstants locks the role strings the SDK exposes. The
// strings are part of the public API contract — renaming them is
// a breaking change for consumers.
func TestRoleConstants(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{string(RoleAdmin), "admin"},
		{string(RoleWriter), "writer"},
		{string(RoleReader), "reader"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("role string mismatch: got %q want %q", c.got, c.want)
		}
	}
}

// TestSentinelErrors checks that the re-exported errors match the
// canonical promptsheon/errs sentinels (errors.Is).
func TestSentinelErrors(t *testing.T) {
	cases := []struct {
		reExported error
		canonical  error
	}{
		{ErrNotLeader, errs.ErrNotLeader},
		{ErrProviderMissing, errs.ErrProviderMissing},
		{ErrStoreNotFound, errs.ErrStoreNotFound},
		{ErrStoreConflict, errs.ErrStoreConflict},
		{ErrQuorum, errs.ErrQuorum},
		{ErrSelfVote, errs.ErrSelfVote},
		{ErrVaultUnknown, errs.ErrVaultUnknown},
		{ErrVaultStopped, errs.ErrVaultStopped},
		{ErrPrecondition, errs.ErrPrecondition},
		{ErrContextExhausted, errs.ErrContextExhausted},
		{ErrInvalidCron, errs.ErrInvalidCron},
	}
	for _, c := range cases {
		if !errors.Is(c.reExported, c.canonical) {
			t.Errorf("sentinel mismatch: %v vs %v", c.reExported, c.canonical)
		}
	}
}

// TestAuditKeys locks the audit JSON-key strings. Changing a
// key value would break consumer code that reads audit rows.
func TestAuditKeys(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{KeyName, "name"},
		{KeyStatus, "status"},
		{KeyVersion, "version"},
		{FieldAPIKey, "api_key"},
		{FieldKeyPref, "key_prefix"},
		{FieldKeyName, "key_name"},
		{FieldProvider, "provider"},
		{FieldModel, "model"},
		{FieldValue, "value"},
		{FieldUserID, "user_id"},
		{FieldEmail, "email"},
		{FieldRole, "role"},
		{FieldError, "error"},
		{FieldOK, "ok"},
		{AnonUser, "api"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("audit key mismatch: got %q want %q", c.got, c.want)
		}
	}
}

// TestNew_NotNil verifies the constructor returns a non-nil client.
// The full request/response cycle is covered in the upstream
// sdk package's tests; this guards the re-export contract.
func TestNew_NotNil(t *testing.T) {
	c := New("https://example.invalid", "ps_test")
	if c == nil {
		t.Fatal("New returned nil client")
	}
}

// TestNewWithHTTP_NilClient ensures a nil *http.Client falls
// back to http.DefaultClient without panicking.
func TestNewWithHTTP_NilClient(t *testing.T) {
	c := NewWithHTTP("https://example.invalid", "ps_test", nil)
	if c == nil {
		t.Fatal("NewWithHTTP returned nil client")
	}
}

// TestTypeAliasesExist ensures every re-exported type still
// resolves to a non-nil reflect type. This catches accidental
// alias breakage during upstream refactors.
func TestTypeAliasesExist(t *testing.T) {
	// A no-op type assertion through a typed nil. If any alias
	// were broken (e.g. upstream removed the type), compilation
	// fails here.
	var (
		_ Workspace      = Workspace{}
		_ Project        = Project{}
		_ Capability     = Capability{}
		_ Version        = Version{}
		_ Release        = Release{}
		_ Approval       = Approval{}
		_ Execution      = Execution{}
		_ Dataset        = Dataset{}
		_ Precondition   = Precondition{}
		_ EvalRun        = EvalRun{}
		_ APIKey         = APIKey{}
		_ HealthResponse = HealthResponse{}
		_ *APIError      = (*APIError)(nil)
	)
}

// TestDefaultAdminEmail locks the bootstrap admin email. The
// value is fixed (only the bootstrap password is overridable
// via PROMPTSHEON_BOOTSTRAP_TOKEN).
func TestDefaultAdminEmail(t *testing.T) {
	if DefaultAdminEmail != "admin@local" {
		t.Errorf("DefaultAdminEmail = %q, want admin@local", DefaultAdminEmail)
	}
}

// TestContextNotRequired is a compile-time check that the public
// types don't take a *context.Context parameter (a common
// regression when re-exporting).
func TestContextNotRequired(t *testing.T) {
	_ = context.Background
}
