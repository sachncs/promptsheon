package sqliteimpl_test

import (
	"context"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/recommendation"
	"github.com/sachncs/promptsheon/promptsheon/store"

	. "github.com/sachncs/promptsheon/promptsheon/store/sqliteimpl"
)

func openTestDB(t *testing.T) *store.SQLite {
	t.Helper()
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	db, err := store.NewSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedVersion(t *testing.T, db *store.SQLite) (*capability.Capability, *capability.Version) {
	t.Helper()
	ctx := context.Background()
	w := &capability.Workspace{ID: "w", Name: "workspace"}
	p := &capability.Project{ID: "p", WorkspaceID: w.ID, Name: "project"}
	c := &capability.Capability{ID: "c", ProjectID: p.ID, Name: "capability", Owner: "owner"}
	v := &capability.Version{ID: "v", CapabilityID: c.ID, Version: 1}
	for _, err := range []error{db.CreateWorkspace(ctx, w), db.CreateProject(ctx, p), db.CreateCapability(ctx, c), db.CreateVersion(ctx, v)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	return c, v
}

func TestRecommendationRepositoryPersists(t *testing.T) {
	db := openTestDB(t)
	_, version := seedVersion(t, db)
	repo := NewRecommendationRepository(db.DB())
	rec := &capability.Recommendation{ID: "r", CapabilityVersionID: version.ID, Type: capability.RecommendationType("test")}
	if err := repo.CreateRecommendation(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetRecommendation(context.Background(), rec.ID)
	if err != nil || got.ID != rec.ID {
		t.Fatalf("got %#v, %v", got, err)
	}
	d := &recommendation.Decision{ID: "d", RecommendationID: rec.ID, DecidedAt: time.Now()}
	if err := repo.CreateDecision(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.GetDecision(context.Background(), rec.ID); err != nil || got.ID != d.ID {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestLineageRepositoryPersistsGraph(t *testing.T) {
	t.Skip("promptsheon/lineage package removed; lineage persistence is no longer wired")
}
