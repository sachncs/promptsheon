package api

import "encoding/json"

// mustMarshalNoArgs returns the JSON encoding of v. The package's
// tests define a mustMarshal(t, v) variant for test ergonomics;
// the production path uses this no-arg variant. The reason for
// two helpers is that the test version captures *testing.T for
// t.Helper() while the production path doesn't.
func mustMarshalNoArgs(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
