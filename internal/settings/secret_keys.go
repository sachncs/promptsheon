package settings

import "strings"

// IsSecretKey reports whether key contains a secret-bearing segment.
// A sensitive segment immediately followed by "ref" names a reference,
// not secret material (for example llm.openai.api_key_ref).
func IsSecretKey(key string) bool {
	segments := strings.FieldsFunc(strings.ToLower(key), func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	for i, segment := range segments {
		switch segment {
		case "key", "secret", "password", "token":
			if i+1 >= len(segments) || segments[i+1] != "ref" {
				return true
			}
		}
	}
	return false
}
