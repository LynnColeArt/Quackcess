package canvasservice

import (
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCanvasServiceSaveAndRenameBumpVersion(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "canvas-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := catalog.NewCanvasRepository(database.SQL)
	service := NewCanvasArtifactService(repo)

	if _, err := service.CreateDraftCanvas("agent-workspace"); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	err = service.SaveCanvasSpec(
		"agent-workspace",
		`{"nodes":[{"id":"n1","table":"events","kind":"table","fields":[{"name":"id"},{"name":"name"}],"alias":"e"}],"edges":[]}`,
		"agent",
	)
	if err != nil {
		t.Fatalf("save canvas spec: %v", err)
	}

	loaded, err := repo.FindByName("agent-workspace")
	if err != nil {
		t.Fatalf("find canvas: %v", err)
	}
	if loaded.Version != 2 {
		t.Fatalf("version after save = %d, want 2", loaded.Version)
	}
	if loaded.SourceRef != "agent" {
		t.Fatalf("source ref = %q, want agent", loaded.SourceRef)
	}

	if err := service.RenameCanvas("agent-workspace", "agent-memory"); err != nil {
		t.Fatalf("rename canvas: %v", err)
	}
	renamed, err := repo.FindByName("agent-memory")
	if err != nil {
		t.Fatalf("find renamed canvas: %v", err)
	}
	if renamed.Version != 3 {
		t.Fatalf("version after rename = %d, want 3", renamed.Version)
	}
}
