package sqliteimpl

import (
	"context"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/lineage"
	"github.com/sachncs/promptsheon/internal/recommendation"
	"github.com/sachncs/promptsheon/internal/store"
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
	db := openTestDB(t)
	c, _ := seedVersion(t, db)
	v2 := &capability.Version{ID: "v2", CapabilityID: c.ID, Version: 2}
	if err := db.CreateVersion(context.Background(), v2); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	g, err := (lineage.Graph{CapabilityID: c.ID}).AppendRecommendation(lineage.VersionRef{CapabilityID: c.ID, Version: 1}, lineage.VersionRef{CapabilityID: c.ID, Version: 2}, lineage.SourceManual, "", "user", "notes", at)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewLineageRepository(db.DB())
	err = repo.PutGraph(context.Background(), &g)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetGraph(context.Background(), c.ID)
	if err != nil || len(got.Edges) != 1 || got.Edges[0].Child.Version != 2 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
