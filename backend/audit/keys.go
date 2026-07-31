// Package audit — centralised constants for the audit log.
//
// Field-key strings used across handler and store layers. Kept in
// one place so renames don't fan out and so forensic analysts
// have a single key alphabet to grep.
package audit

// AnonUser is the user ID recorded on audit rows when no caller
// is in scope (boot, unauthenticated routes, the auth-disabled
// loopback profile).
const AnonUser = "api"

// Field key names. The values are the on-the-wire JSON keys in
// the audit entry's Details map and in CSV exports.
const (
	KeyName       = "name"
	KeyStatus     = "status"
	KeyVersion    = "version"
	FieldAPIKey   = "api_key" // was "key"; renamed to be unambiguous
	FieldKeyPref  = "key_prefix"
	FieldKeyName  = "key_name"
	FieldProvider = "provider"
	// FieldProviderName is the human-friendly name of the provider
	// (e.g. "openai-production"). Distinct from FieldProvider
	// which is the machine identifier ("openai").
	FieldProviderName = "provider_name"
	FieldModel    = "model"
	FieldValue    = "value"
	FieldUserID   = "user_id"
	FieldEmail    = "email"
	FieldRole     = "role"
	FieldError    = "error"
	FieldOK       = "ok"
)