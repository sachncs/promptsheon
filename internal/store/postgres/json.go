package postgres

import "encoding/json"

// jsonMarshal / jsonUnmarshal are tiny shims so this package can
// stay minimal and avoid re-importing encoding/json in every
// helper file. The shapes (capability.Manifest etc.) are JSON-safe.
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func jsonUnmarshal(b []byte, v any) error {
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, v)
}
