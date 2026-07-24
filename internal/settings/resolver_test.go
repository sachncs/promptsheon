// Package settings — Resolver tests. The three-layer
// precedence (env > DB > default) is the production contract;
// the tests below pin it so a future refactor doesn't flip
// the order by accident. The CRDT-specific behaviour
// (vector dominance, tombstone) lives in crdt_test.go.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type stubStore struct {
	rows map[string]CRDTRecord
}

func (s *stubStore) GetSystemConfig(_ context.Context, key string) (CRDTRecord, error) {
	rec, ok := s.rows[key]
	if !ok {
		return CRDTRecord{}, sql.ErrNoRows
	}
	return rec, nil
}

func (s *stubStore) SetSystemConfig(_ context.Context, rec CRDTRecord) error {
	if s.rows == nil {
		s.rows = map[string]CRDTRecord{}
	}
	s.rows[rec.Key] = rec
	return nil
}

func (s *stubStore) ListSystemConfig(_ context.Context) ([]CRDTRecord, error) {
	out := make([]CRDTRecord, 0, len(s.rows))
	for _, v := range s.rows {
		out = append(out, v)
	}
	return out, nil
}

func (s *stubStore) MergeSystemConfig(_ context.Context, replicaID string, records []CRDTRecord) error {
	if s.rows == nil {
		s.rows = map[string]CRDTRecord{}
	}
	for _, r := range records {
		existing, ok := s.rows[r.Key]
		if !ok {
			existing = CRDTRecord{Key: r.Key}
		}
		merged := Merge(existing, r)
		if merged.ReplicaID == "" {
			merged.ReplicaID = replicaID
		}
		s.rows[r.Key] = merged
	}
	return nil
}

type stubEnv struct{ m map[string]string }

func (s stubEnv) Get(k string) (string, bool) { v, ok := s.m[k]; return v, ok }

func TestResolver_Precedence_EnvWinsOverDB(t *testing.T) {
	t.Parallel()
	s := &stubStore{rows: map[string]CRDTRecord{
		"otl.endpoint": {Key: "otl.endpoint", Value: "http://from-db:4317"},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{
		"otl.endpoint": "http://from-env:4317",
	}}, "rep-a")
	got := r.Get(context.Background(), "otl.endpoint")
	if got != "http://from-env:4317" {
		t.Fatalf("env-floor violated: got %q, want %q", got, "http://from-env:4317")
	}
}

func TestResolver_Precedence_DBIsCeiling(t *testing.T) {
	t.Parallel()
	s := &stubStore{rows: map[string]CRDTRecord{
		"otl.endpoint": {Key: "otl.endpoint", Value: "http://from-db:4317"},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{}}, "rep-a")
	got := r.Get(context.Background(), "otl.endpoint")
	if got != "http://from-db:4317" {
		t.Fatalf("DB-ceiling violated: got %q, want %q", got, "http://from-db:4317")
	}
}

func TestResolver_Precedence_Default(t *testing.T) {
	t.Parallel()
	r := NewResolver(&stubStore{}, nil, stubEnv{m: map[string]string{}}, "rep-a")
	if got := r.Get(context.Background(), "otl.endpoint"); got != "" {
		t.Fatalf("default should be empty, got %q", got)
	}
}

func TestResolver_DeleteReassertsEnv(t *testing.T) {
	t.Parallel()
	s := &stubStore{rows: map[string]CRDTRecord{
		"otl.endpoint": {Key: "otl.endpoint", Value: "http://from-db:4317"},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{
		"otl.endpoint": "http://from-env:4317",
	}}, "rep-a")
	if err := r.Delete(context.Background(), "otl.endpoint"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := r.Get(context.Background(), "otl.endpoint"); got != "http://from-env:4317" {
		t.Fatalf("post-delete env-floor: got %q, want %q", got, "http://from-env:4317")
	}
}

func TestResolver_NotifierFiresOnSet(t *testing.T) {
	t.Parallel()
	n := NewNotifier()
	var got string
	n.Subscribe("otl.endpoint", func(v string) error { got = v; return nil })
	s := &stubStore{}
	r := NewResolver(s, n, stubEnv{m: map[string]string{}}, "rep-a")
	if err := r.Set(context.Background(), "otl.endpoint", "http://new:4317", "tester"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got != "http://new:4317" {
		t.Fatalf("notifier not fired, got %q", got)
	}
}

func TestResolver_NotifierErrorPropagates(t *testing.T) {
	t.Parallel()
	n := NewNotifier()
	n.Subscribe("otl.endpoint", func(_ string) error { return errors.New("reload failed") })
	s := &stubStore{}
	r := NewResolver(s, n, stubEnv{m: map[string]string{}}, "rep-a")
	if err := r.Set(context.Background(), "otl.endpoint", "http://new:4317", "tester"); err == nil {
		t.Fatal("expected notifier error to propagate")
	}
}

// TestResolver_DeleteMissingWritesTombstone pins the new
// CRDT contract: deleting a key that does not exist is not an
// error — the resolver writes a tombstone row so a concurrent
// replica's Set on the same key cannot resurrect the row.
// sql.ErrNoRows is no longer surfaced because the tombstone
// write always succeeds.
func TestResolver_DeleteMissingWritesTombstone(t *testing.T) {
	t.Parallel()
	s := &stubStore{}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{}}, "rep-a")
	if err := r.Delete(context.Background(), "missing"); err != nil {
		t.Fatalf("delete missing: unexpected error %v", err)
	}
	rec, ok := s.rows["missing"]
	if !ok {
		t.Fatal("expected tombstone row after delete")
	}
	if !rec.Tombstone {
		t.Fatalf("expected tombstone flag, got %+v", rec)
	}
	if rec.ReplicaID != "rep-a" {
		t.Fatalf("expected rep-a attribution, got %q", rec.ReplicaID)
	}
}

func TestSecretKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"vault.key", "webhook.signing_secret", "db.password", "oauth-token"} {
		if !IsSecretKey(key) {
			t.Fatalf("%q should be secret", key)
		}
	}
	for _, key := range []string{"nonexistent", "llm.openai.api_key_ref", "oauth.token_ref"} {
		if IsSecretKey(key) {
			t.Fatalf("%q should not be secret", key)
		}
	}
}

// TestResolver_ListHidesTombstones pins the Get/List
// semantics: a tombstoned row is invisible to API consumers.
func TestResolver_ListHidesTombstones(t *testing.T) {
	t.Parallel()
	ts := time.Now().UTC()
	s := &stubStore{rows: map[string]CRDTRecord{
		"otl.endpoint":   {Key: "otl.endpoint", Value: "http://from-db:4317", WriteTS: ts.UnixNano()},
		"otl.deprecated": {Key: "otl.deprecated", Value: "", Tombstone: true, WriteTS: ts.UnixNano() + 1},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{}}, "rep-a")
	rows, err := r.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, rec := range rows {
		if rec.Tombstone {
			t.Fatalf("tombstone leaked: %+v", rec)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 visible row, got %d", len(rows))
	}
}

// TestResolver_SetBumpsLocalVector pins the increment-on-Set
// contract: every Set from the local replica bumps the local
// replica's vector entry by one. Concurrent replicas will
// detect the local write via their own merge pass.
func TestResolver_SetBumpsLocalVector(t *testing.T) {
	t.Parallel()
	s := &stubStore{}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{}}, "rep-a")
	if err := r.Set(context.Background(), "k", "v1", "tester"); err != nil {
		t.Fatal(err)
	}
	if err := r.Set(context.Background(), "k", "v2", "tester"); err != nil {
		t.Fatal(err)
	}
	rec, _ := s.GetSystemConfig(context.Background(), "k")
	if rec.VersionVector["rep-a"] != 2 {
		t.Fatalf("expected rep-a=2, got %d", rec.VersionVector["rep-a"])
	}
}

// TestResolver_MergeInvokesStore pins the MergeSystemConfig
// call: the Resolver forwards records to the Store.
func TestResolver_MergeInvokesStore(t *testing.T) {
	t.Parallel()
	s := &stubStore{}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{}}, "rep-a")
	remote := []CRDTRecord{{Key: "k", Value: "from-b", ReplicaID: "rep-b", WriteTS: 100, VersionVector: map[string]uint64{"rep-b": 1}}}
	if err := r.Merge(context.Background(), "rep-b", remote); err != nil {
		t.Fatal(err)
	}
	rec, _ := s.GetSystemConfig(context.Background(), "k")
	if rec.Value != "from-b" {
		t.Fatalf("expected from-b, got %+v", rec)
	}
}
