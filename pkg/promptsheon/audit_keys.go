//go:build promptsheon

package promptsheon

import (
	"github.com/sachncs/promptsheon/promptsheon"
)

// Re-exported audit constants. These are the JSON keys used in
// audit row Details maps. Consumer code that reads audit entries
// should reference these constants rather than hard-coding the
// strings.
const (
	KeyName           = audit.KeyName
	KeyStatus         = audit.KeyStatus
	KeyVersion        = audit.KeyVersion
	FieldAPIKey       = audit.FieldAPIKey
	FieldKeyPref      = audit.FieldKeyPref
	FieldKeyName      = audit.FieldKeyName
	FieldProvider     = audit.FieldProvider
	FieldProviderName = audit.FieldProviderName
	FieldModel        = audit.FieldModel
	FieldValue        = audit.FieldValue
	FieldUserID       = audit.FieldUserID
	FieldEmail        = audit.FieldEmail
	FieldRole         = audit.FieldRole
	FieldError        = audit.FieldError
	FieldOK           = audit.FieldOK
	AnonUser          = audit.AnonUser
)
