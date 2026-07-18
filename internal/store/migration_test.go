package store_test

import (
	"testing"

	"github.com/sachncs/promptsheon/internal/store"
)

func TestReleasesMigration024(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	rows, err := db.DB().Query("SELECT name FROM sqlite_master WHERE type='table' AND name IN ('releases','approvals') ORDER BY name")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, n)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tables (releases, approvals), got %v", got)
	}
}

func TestHarnessMigration025(t *testing.T) {
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	want := map[string]bool{
		"datasets":      false,
		"dataset_cases": false,
		"preconditions": false,
		"eval_runs":     false,
		"eval_results":  false,
	}
	rows, err := db.DB().Query(
		"SELECT name FROM sqlite_master WHERE type='table' AND name IN ('datasets','dataset_cases','preconditions','eval_runs','eval_results')")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, present := range want {
		if !present {
			t.Fatalf("expected table %q to exist", n)
		}
	}
}
