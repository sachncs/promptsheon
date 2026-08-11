//go:build promptsheon

package promptsheon

// Re-exported audit constants. These are the JSON keys used in
// audit row Details maps. Consumer code that reads audit entries
// should reference these constants rather than hard-coding the
// strings.
//
// The string values are duplicated from promptsheon/audit.go;
// that file lives in package promptsheon without a build tag, so
// re-exporting via assignment (KeyName = KeyName) would conflict.
// The values are pinned by TestAuditKeys in public_test.go.
const (
	KeyName           = "name"
	KeyStatus         = "status"
	KeyVersion        = "version"
	FieldAPIKey       = "api_key"
	FieldKeyPref      = "key_prefix"
	FieldKeyName      = "key_name"
	FieldProvider     = "provider"
	FieldProviderName = "provider_name"
	FieldModel        = "model"
	FieldValue        = "value"
	FieldUserID       = "user_id"
	FieldEmail        = "email"
	FieldRole         = "role"
	FieldError        = "error"
	FieldOK           = "ok"
	AnonUser          = "api"
)
