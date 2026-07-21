package harness

import (
	"testing"
)

// TestScrubEnvAllowlistPasses locks in the SEC-2a half of the
// acceptance: HOME/PATH/LANG pass through scrubEnv unchanged.
func TestScrubEnvAllowlistPasses(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("HOME", "/home/x")
	t.Setenv("LANG", "en_US.UTF-8")
	out := scrubEnv()
	got := map[string]string{}
	for _, kv := range out {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				got[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	for _, k := range []string{"PATH", "HOME", "LANG"} {
		if _, ok := got[k]; !ok {
			t.Errorf("expected %s in scrubEnv output, got %v", k, got)
		}
	}
}

// TestScrubEnvDenylistStrips locks in the other half of the
// SEC-2a acceptance: a credential-shaped variable (prefix
// match AWS_, suffix match _KEY) is removed.
func TestScrubEnvDenylistStrips(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA-secret")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("PATH_SECRET", "leak") // _SECRET suffix match
	out := scrubEnv()
	for _, kv := range out {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				name := kv[:i]
				switch name {
				case "AWS_ACCESS_KEY_ID", "OPENAI_API_KEY", "PATH_SECRET":
					t.Errorf("denylist failed: %s leaked", name)
				}
				break
			}
		}
	}
}
