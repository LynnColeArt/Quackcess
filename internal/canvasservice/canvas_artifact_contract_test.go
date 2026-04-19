package canvasservice

import (
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCanvasArtifactServiceCreateDraftCanvas(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "canvas-artifact-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := catalog.NewCanvasRepository(database.SQL)
	service := NewCanvasArtifactService(repo)

	canvas, err := service.CreateDraftCanvas("Sales Overview")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if canvas.Name != "Sales Overview" {
		t.Fatalf("name = %q, want Sales Overview", canvas.Name)
	}
	if canvas.Version != 1 {
		t.Fatalf("version = %d, want 1", canvas.Version)
	}

	reloaded, err := repo.FindByName("Sales Overview")
	if err != nil {
		t.Fatalf("find reloaded: %v", err)
	}
	if reloaded.Version != 1 {
		t.Fatalf("reloaded version = %d, want 1", reloaded.Version)
	}
	if reloaded.SpecJSON != `{"nodes":[],"edges":[]}` && reloaded.SpecJSON != `{"edges":[],"nodes":[]}` {
		t.Fatalf("unexpected draft spec json: %q", reloaded.SpecJSON)
	}
}

func TestCanvasArtifactServiceHistoryHelper(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "canvas-history.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := catalog.NewCanvasRepository(database.SQL)
	service := NewCanvasArtifactService(repo)
	if err := repo.Create(catalog.Canvas{
		ID:       "canvas-history",
		Name:     "history-test",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"t","table":"users","fields":[{"name":"id"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	history, err := service.History("history-test")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].Name != "history-test" {
		t.Fatalf("history name = %q, want history-test", history[0].Name)
	}
	if history[0].Version != 1 {
		t.Fatalf("history version = %d, want 1", history[0].Version)
	}
	if history[0].ID != "canvas-history" {
		t.Fatalf("history id = %q, want canvas-history", history[0].ID)
	}
}

func TestCanvasArtifactServiceGetForExecutionNormalizesSpec(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "canvas-execution.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := catalog.NewCanvasRepository(database.SQL)
	service := NewCanvasArtifactService(repo)
	if err := repo.Create(catalog.Canvas{
		ID:       "canvas-exec",
		Name:     "exec",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"t","table":"users","kind":"table","alias":"u","fields":[{"name":"id"}],"x":-10}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	spec, err := service.GetForExecution("exec")
	if err != nil {
		t.Fatalf("get for execution: %v", err)
	}
	if spec.Nodes[0].Alias != "u" {
		t.Fatalf("alias = %q, want u", spec.Nodes[0].Alias)
	}
	if spec.Nodes[0].X != 0 {
		t.Fatalf("x = %v, want 0", spec.Nodes[0].X)
	}
}
