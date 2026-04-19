package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactPackUnpackPreservesKindedArtifactPaths(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := writeFileForTest(dbPath, []byte("duckdb-placeholder")); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "pack-unpack.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "ArtifactPack"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindCanvas, "canvas-main"):       []byte(`{"id":"canvas-main","kind":"canvas","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindQuery, "query-main"):         []byte(`{"id":"query-main","kind":"query","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "chart-main"):         []byte(`{"id":"chart-main","kind":"chart","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "report-main"):       []byte(`{"id":"report-main","kind":"report","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindProcedure, "procedure-main"): []byte(`{"id":"procedure-main","kind":"procedure","schemaVersion":"1.0.0"}`),
	}

	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: dbPath,
		Artifacts:          artifacts,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	contents, err := project.Contents()
	if err != nil {
		t.Fatalf("list contents: %v", err)
	}
	for _, expected := range []string{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindCanvas, "canvas-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindQuery, "query-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "chart-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "report-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindProcedure, "procedure-main"),
	} {
		if !containsString(contents, expected) {
			t.Fatalf("contents missing %q", expected)
		}
	}

	for _, expected := range []string{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindCanvas, "canvas-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindQuery, "query-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "chart-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "report-main"),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindProcedure, "procedure-main"),
	} {
		payload, err := project.ReadArtifact(expected)
		if err != nil {
			t.Fatalf("read artifact %q: %v", expected, err)
		}
		if !strings.Contains(string(payload), `"schemaVersion":"1.0.0"`) {
			t.Fatalf("payload %q missing schemaVersion", expected)
		}
	}
}

func TestArtifactPackRejectsMalformedArtifactEntries(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := writeFileForTest(dbPath, []byte("duckdb-placeholder")); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "bad-artifacts.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "MalformedArtifacts"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		"../escape/config.json": []byte(`{"bad":"entry"}`),
	}

	err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: dbPath,
		Artifacts:          artifacts,
	})
	if err == nil {
		t.Fatal("expected malformed artifact error")
	}
	if !strings.Contains(err.Error(), "invalid artifact path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeFileForTest(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
