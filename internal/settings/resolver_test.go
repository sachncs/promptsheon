// Package settings — Resolver tests. The three-layer
// precedence (env > DB > default) is the production contract;
// the tests below pin it so a future refactor doesn't flip
// the order by accident.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type stubStore struct {
	rows map[string]struct {
		value     string
		updatedAt time.Time
		by        string
	}
}

func (s *stubStore) GetSystemConfig(_ context.Context, key string) (string, time.Time, error) {
	r, ok := s.rows[key]
	if !ok {
		return "", time.Time{}, sql.ErrNoRows
	}
	return r.value, r.updatedAt, nil
}

func (s *stubStore) SetSystemConfig(_ context.Context, key, value, by string) error {
	if s.rows == nil {
		s.rows = map[string]struct {
			value     string
			updatedAt time.Time
			by        string
		}{}
	}
	s.rows[key] = struct {
		value     string
		updatedAt time.Time
		by        string
	}{value, time.Now(), by}
	return nil
}

func (s *stubStore) DeleteSystemConfig(_ context.Context, key string) error {
	if _, ok := s.rows[key]; !ok {
		return sql.ErrNoRows
	}
	delete(s.rows, key)
	return nil
}

func (s *stubStore) ListSystemConfig(_ context.Context) ([]SystemConfigRow, error) {
	out := make([]SystemConfigRow, 0, len(s.rows))
	for k, v := range s.rows {
		out = append(out, SystemConfigRow{Key: k, Value: v.value, UpdatedAt: v.updatedAt, UpdatedBy: v.by})
	}
	return out, nil
}

type stubEnv struct{ m map[string]string }

func (s stubEnv) Get(k string) (string, bool) { v, ok := s.m[k]; return v, ok }

func TestResolver_Precedence_EnvWinsOverDB(t *testing.T) {
	t.Parallel()
	s := &stubStore{rows: map[string]struct {
		value     string
		updatedAt time.Time
		by        string
	}{
		"otl.endpoint": {value: "http://from-db:4317"},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{
		"otl.endpoint": "http://from-env:4317",
	}})
	got := r.Get(context.Background(), "otl.endpoint")
	if got != "http://from-env:4317" {
		t.Fatalf("env-floor violated: got %q, want %q", got, "http://from-env:4317")
	}
}

func TestResolver_Precedence_DBIsCeiling(t *testing.T) {
	t.Parallel()
	s := &stubStore{rows: map[string]struct {
		value     string
		updatedAt time.Time
		by        string
	}{
		"otl.endpoint": {value: "http://from-db:4317"},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{}})
	got := r.Get(context.Background(), "otl.endpoint")
	if got != "http://from-db:4317" {
		t.Fatalf("DB-ceiling violated: got %q, want %q", got, "http://from-db:4317")
	}
}

func TestResolver_Precedence_Default(t *testing.T) {
	t.Parallel()
	r := NewResolver(&stubStore{}, nil, stubEnv{m: map[string]string{}})
	if got := r.Get(context.Background(), "otl.endpoint"); got != "" {
		t.Fatalf("default should be empty, got %q", got)
	}
}

func TestResolver_DeleteReassertsEnv(t *testing.T) {
	t.Parallel()
	s := &stubStore{rows: map[string]struct {
		value     string
		updatedAt time.Time
		by        string
	}{
		"otl.endpoint": {value: "http://from-db:4317"},
	}}
	r := NewResolver(s, nil, stubEnv{m: map[string]string{
		"otl.endpoint": "http://from-env:4317",
	}})
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
	n.Subscribe("otl.endpoint", func(v string) { got = v })
	s := &stubStore{}
	r := NewResolver(s, n, stubEnv{m: map[string]string{}})
	if err := r.Set(context.Background(), "otl.endpoint", "http://new:4317", "tester"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got != "http://new:4317" {
		t.Fatalf("notifier not fired, got %q", got)
	}
}

func TestResolver_NotFoundError(t *testing.T) {
	t.Parallel()
	r := NewResolver(&stubStore{}, nil, stubEnv{m: map[string]string{}})
	err := r.Delete(context.Background(), "missing")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("delete missing: want sql.ErrNoRows, got %v", err)
	}
}

func TestSecretKeys(t *testing.T) {
	t.Parallel()
	if IsSecretKey("nonexistent") {
		t.Fatal("nonexistent should not be secret")
	}
	RegisterSecretKey("test.secret")
	if !IsSecretKey("test.secret") {
		t.Fatal("registered key should be secret")
	}
}
