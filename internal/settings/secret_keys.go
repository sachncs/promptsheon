// Package settings — keys whose value must be masked in the
// API response. Defaults to empty; future secret-shaped keys
// (webhook signing secret, SAML secret, etc.) add themselves
// here as they ship.
package settings

// secretKeys is the read-time mask list. The API layer reads
// it when formatting the GET response; if a key is in the
// list, the value is replaced with "***" before the response
// is serialised.
var secretKeys = map[string]bool{}

// IsSecretKey reports whether the key is in the read-time
// mask list. Used by the API layer's GET handler.
func IsSecretKey(key string) bool { return secretKeys[key] }

// RegisterSecretKey adds a key to the read-time mask list.
// Future secret-shaped keys (e.g. `webhook.signing_secret`)
// call this at init time. Public so packages shipping the
// secret can self-register.
func RegisterSecretKey(key string) { secretKeys[key] = true }
