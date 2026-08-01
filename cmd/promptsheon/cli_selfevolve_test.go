package main

import (
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCmdSelfEvolve_Help(t *testing.T) {
	if err := cmdSelfEvolve([]string{}); err == nil {
		t.Errorf("expected error for empty args")
	}
}

func TestSelfEvolveParseFlags(t *testing.T) {
	flags, values, err := selfEvolveParseFlags([]string{
		"--dataset=ds-1", "--min-score=0.8", "--max-revisions", "5", "--target-env", "staging", "--force",
	})
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if values["dataset"] != "ds-1" {
		t.Errorf("dataset = %q", values["dataset"])
	}
	if values["min-score"] != "0.8" {
		t.Errorf("min-score = %q", values["min-score"])
	}
	if values["max-revisions"] != "5" {
		t.Errorf("max-revisions = %q", values["max-revisions"])
	}
	if values["target-env"] != "staging" {
		t.Errorf("target-env = %q", values["target-env"])
	}
	if !flags["force"] {
		t.Errorf("boolean flag 'force' not set: %+v", flags)
	}
}

func TestSelfEvolveParseFlags_Positional(t *testing.T) {
	if _, _, err := selfEvolveParseFlags([]string{"ds-1"}); err == nil {
		t.Errorf("expected error for positional arg")
	}
}

func TestCmdSelfEvolve_EnableDisableStatus(t *testing.T) {
	// Spin up a fake daemon that captures the requests.
	var lastMethod string
	var lastPath string
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		if r.URL.Path == "/api/v1/capabilities/c1" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"c1","self_evolve":{"enabled":true,"min_score":0.9,"max_revisions":10,"cooldown_sec":900,"target_env":"dev","dataset_id":"ds-1"}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("PROMPTSHEON_SERVER", server.URL)

	// status reads the capability.
	if err := cmdSelfEvolve([]string{"status", "c1"}); err != nil {
		t.Fatalf("status: %v", err)
	}
	if lastMethod != "GET" {
		t.Errorf("status method = %q, want GET", lastMethod)
	}

	// enable PUTs the config.
	if err := cmdSelfEvolve([]string{"enable", "c1", "--dataset", "ds-1", "--min-score", "0.85"}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if lastMethod != "PUT" {
		t.Errorf("enable method = %q, want PUT", lastMethod)
	}
	if !strings.Contains(lastPath, "self-evolve") {
		t.Errorf("enable path = %q, want /self-evolve", lastPath)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(lastBody), &got); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if got["enabled"] != true {
		t.Errorf("enabled = %v, want true", got["enabled"])
	}
	if got["dataset_id"] != "ds-1" {
		t.Errorf("dataset_id = %v, want ds-1", got["dataset_id"])
	}
	if got["min_score"] != 0.85 {
		t.Errorf("min_score = %v, want 0.85", got["min_score"])
	}

	// disable PUTs enabled=false.
	if err := cmdSelfEvolve([]string{"disable", "c1"}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if lastMethod != "PUT" {
		t.Errorf("disable method = %q", lastMethod)
	}
	got = nil
	json.Unmarshal([]byte(lastBody), &got)
	if got["enabled"] != false {
		t.Errorf("disable enabled = %v, want false", got["enabled"])
	}
}
