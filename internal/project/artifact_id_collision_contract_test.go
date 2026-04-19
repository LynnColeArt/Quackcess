package project

import (
	"path/filepath"
	"strings"
	"testing"

	"os"
)

func TestCreateRejectsCanonicalizedArtifactPathCollisions(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "collision.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "CollisionCase"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		"artifacts/canvas/dup-canvas/manifest.json":     []byte(`{"id":"dup-canvas","kind":"canvas","schemaVersion":"1.0.0"}`),
		"artifacts/canvas/./dup-canvas/./manifest.json": []byte(`{"id":"dup-canvas","kind":"canvas","schemaVersion":"1.0.0"}`),
	}

	err := Create(projectPath, CreateOptions{
		Manifest:  manifest,
		Artifacts: artifacts,
	})
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !strings.Contains(err.Error(), "artifact path collision") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRejectsDuplicateArtifactIdsAcrossKinds(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "distinct.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "DistinctNamespaces"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindCanvas, "shared-id"):    []byte(`{"id":"shared-id","kind":"canvas","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "shared-id"):     []byte(`{"id":"shared-id","kind":"chart","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindQuery, "shared-id"):     []byte(`{"id":"shared-id","kind":"query","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindProcedure, "shared-id"): []byte(`{"id":"shared-id","kind":"procedure","schemaVersion":"1.0.0"}`),
	}

	err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: dbPath,
		Artifacts:          artifacts,
	})
	if err == nil {
		t.Fatal("expected duplicate artifact id error")
	}
	if !strings.Contains(err.Error(), "duplicate artifact id") {
		t.Fatalf("unexpected error: %v", err)
	}
}
