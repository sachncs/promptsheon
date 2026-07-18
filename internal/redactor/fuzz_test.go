package redactor

import "testing"

func FuzzRedact(f *testing.F) {
	f.Add("Contact me at alice@example.com")
	f.Add("Call 555-12-3456 or 5551234567890")
	f.Add("plain text")
	f.Add("")

	f.Fuzz(func(t *testing.T, text string) {
		r := NewRedactor()
		out := r.Redact(text)
		// The redactor must not panic and must always return
		// a string whose length is reasonable.
		if len(out) > 10*len(text)+1024 {
			t.Errorf("output exploded: input=%d output=%d", len(text), len(out))
		}
	})
}
