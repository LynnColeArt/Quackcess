package catalog

import (
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCanvasRepositoryGetByIDAndUpdate(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "meta.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewCanvasRepository(database.SQL)
	if err := repo.Create(Canvas{
		ID:       "canvas-1",
		Name:     "initial",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	loaded, err := repo.GetByID("canvas-1")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if loaded.Name != "initial" {
		t.Fatalf("name = %q, want initial", loaded.Name)
	}

	if err := repo.Update(Canvas{
		ID:       "canvas-1",
		Name:     "renamed",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	refreshed, err := repo.GetByID("canvas-1")
	if err != nil {
		t.Fatalf("get by id after update: %v", err)
	}
	if refreshed.Name != "renamed" {
		t.Fatalf("updated name = %q, want renamed", refreshed.Name)
	}
	if refreshed.SpecJSON != `{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}` {
		t.Fatalf("updated spec = %s, want expected string", refreshed.SpecJSON)
	}
}

func TestCanvasRepositoryListByKind(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "meta.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewCanvasRepository(database.SQL)
	if err := repo.Create(Canvas{ID: "canvas-1", Name: "customers", Kind: "query", SpecJSON: `{"nodes":[]}`}); err != nil {
		t.Fatalf("create query: %v", err)
	}
	if err := repo.Create(Canvas{ID: "canvas-2", Name: "overview", Kind: "report", SpecJSON: `{"nodes":[]}`}); err != nil {
		t.Fatalf("create report: %v", err)
	}

	canvases, err := repo.ListByKind("query")
	if err != nil {
		t.Fatalf("list by kind: %v", err)
	}
	if len(canvases) != 1 {
		t.Fatalf("len(canvases) = %d, want 1", len(canvases))
	}
	if canvases[0].ID != "canvas-1" {
		t.Fatalf("id = %q, want canvas-1", canvases[0].ID)
	}

	all, err := repo.ListByKind("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all len = %d, want 2", len(all))
	}
}

func TestCanvasRepositoryUpsert(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "meta.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewCanvasRepository(database.SQL)
	if err := repo.Upsert(Canvas{
		ID:       "canvas-upsert",
		Name:     "draft",
		Kind:     "query",
		SpecJSON: `{"nodes":[]}`,
	}); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if err := repo.Upsert(Canvas{
		ID:       "canvas-upsert",
		Name:     "final",
		Kind:     "query",
		SpecJSON: `{"nodes":[],"edges":[]}`,
	}); err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	loaded, err := repo.GetByID("canvas-upsert")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if loaded.Name != "final" {
		t.Fatalf("name = %q, want final", loaded.Name)
	}
	if loaded.SpecJSON != `{"nodes":[],"edges":[]}` {
		t.Fatalf("spec = %s, want %s", loaded.SpecJSON, `{"nodes":[],"edges":[]}`)
	}
}
