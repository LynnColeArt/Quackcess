package catalog

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCatalogCanvasCrudFlow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewCanvasRepository(database.SQL)
	if err := repo.Create(Canvas{
		ID:       "01K9XZQJX3",
		Name:     "sales overview",
		Kind:     "query",
		SpecJSON: `{"nodes":[],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas 1: %v", err)
	}
	if err := repo.Create(Canvas{
		ID:       "01K9XZQJX4",
		Name:     "customer funnel",
		Kind:     "query",
		SpecJSON: `{"nodes":[],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas 2: %v", err)
	}

	canvases, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(canvases) != 2 {
		t.Fatalf("len(canvases) = %d, want 2", len(canvases))
	}
	if canvases[0].Name != "customer funnel" || canvases[1].Name != "sales overview" {
		t.Fatalf("unexpected ordering: %#v", canvases)
	}

	if err := repo.Delete("01K9XZQJX4"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	canvases, err = repo.List()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(canvases) != 1 || canvases[0].ID != "01K9XZQJX3" {
		t.Fatalf("unexpected canvases after delete: %#v", canvases)
	}
}

func TestCatalogCanvasRepositoryRejectsBadSpec(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewCanvasRepository(database.SQL)
	if err := repo.Create(Canvas{}); err == nil {
		t.Fatalf("expected validation error")
	}
	if err := repo.Create(Canvas{ID: "01", Name: "bad", Kind: "query", SpecJSON: ""}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestCatalogCanvasRepositoryRejectsDuplicateID(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewCanvasRepository(database.SQL)
	if err := repo.Create(Canvas{
		ID:       "dup-id",
		Name:     "first",
		Kind:     "query",
		SpecJSON: `{"nodes":[]}`,
	}); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := repo.Create(Canvas{
		ID:       "dup-id",
		Name:     "second",
		Kind:     "query",
		SpecJSON: `{"nodes":[]}`,
	}); err == nil {
		t.Fatal("expected duplicate error")
	} else if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
		!strings.Contains(err.Error(), "already exists") &&
		!strings.Contains(err.Error(), "Duplicate key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
