
package main

import (
	"github.com/sachncs/promptsheon/promptsheon/llm"
	"context"
	"testing"

	"github.com/sachncs/promptsheon/promptsheon/evolve"
)

func TestParseSelfEvolveEntry_Valid(t *testing.T) {
	entry, ok := parseSelfEvolveEntry("cap-1:ds-1:0.9:dev:10:900")
	if !ok {
		t.Fatalf("expected ok, got !ok")
	}
	if entry.capID != "cap-1" || entry.datasetID != "ds-1" || entry.targetEnv != "dev" {
		t.Errorf("ids wrong: %+v", entry)
	}
	if entry.threshold != 0.9 {
		t.Errorf("threshold = %v, want 0.9", entry.threshold)
	}
	if entry.maxRevisions != 10 || entry.cooldownSec != 900 {
		t.Errorf("counts wrong: %+v", entry)
	}
}

func TestParseSelfEvolveEntry_BadShape(t *testing.T) {
	cases := []string{
		"",              // empty
		"a:b:c:d:e",     // only 5 parts
		"a:b:c:d:e:f:g", // 7 parts
	}
	for _, c := range cases {
		if _, ok := parseSelfEvolveEntry(c); ok {
			t.Errorf("parseSelfEvolveEntry(%q): expected !ok", c)
		}
	}
}

func TestParseSelfEvolveEntry_BadFields(t *testing.T) {
	cases := []string{
		":ds:0.9:dev:10:900",       // empty cap id
		"cap::0.9:dev:10:900",      // empty ds
		"cap:ds:notnum:dev:10:900", // bad threshold
		"cap:ds:0.9::10:900",       // empty env
		"cap:ds:0.9:dev:0:900",     // zero max
		"cap:ds:0.9:dev:-1:900",    // negative max
		"cap:ds:0.9:dev:10:-1",     // negative cooldown
	}
	for _, c := range cases {
		if _, ok := parseSelfEvolveEntry(c); ok {
			t.Errorf("parseSelfEvolveEntry(%q): expected !ok", c)
		}
	}
}

func TestSplitEntries(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a;b;c", []string{"a", "b", "c"}},
		{"a;;b", []string{"a", "b"}},
		{"a;", []string{"a"}},
		{";a", []string{"a"}},
		{";;", nil},
	}
	for _, c := range cases {
		got := splitEntries(c.in, ';')
		if len(got) != len(c.want) {
			t.Errorf("splitEntries(%q) len = %d, want %d", c.in, len(got), len(c.want))
			continue
		}
		for i, w := range c.want {
			if got[i] != w {
				t.Errorf("splitEntries(%q)[%d] = %q, want %q", c.in, i, got[i], w)
			}
		}
	}
}

func TestMakeEvolverLLMInvoke_NoProviders(t *testing.T) {
	// Construct a registry with zero providers. The closure
	// should return an error when called.
	r := llm.NewRegistry()
	invoke := makeEvolverLLMInvoke(r, nil)
	_, err := invoke(context.Background(), evolve.LLMInvokeRequest{System: "s", User: "u"})
	if err == nil {
		t.Errorf("expected error for no providers")
	}
}
