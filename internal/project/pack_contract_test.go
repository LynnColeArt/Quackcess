package project

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

func TestPackUnpackProjectRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "demo.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "Demo"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		"artifacts/query/query1/manifest.json": []byte(`{"id":"query1","kind":"query","schemaVersion":"1.0.0"}`),
		"artifacts/chart/c1/manifest.json":     []byte(`{"id":"c1","kind":"chart","schemaVersion":"1.0.0"}`),
	}

	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: dbPath,
		Artifacts:          artifacts,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	if p.Manifest.ProjectName != manifest.ProjectName {
		t.Fatalf("project name mismatch: %q != %q", p.Manifest.ProjectName, manifest.ProjectName)
	}

	contents, err := p.Contents()
	if err != nil {
		t.Fatalf("list contents: %v", err)
	}
	for _, want := range []string{
		manifestEntry,
		manifest.DataFile,
		"artifacts/query/query1/manifest.json",
		"artifacts/chart/c1/manifest.json",
	} {
		if !containsString(contents, want) {
			t.Fatalf("contents missing entry %q", want)
		}
	}

	queryPayload, err := p.ReadArtifact("artifacts/query/query1/manifest.json")
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(queryPayload) != string(artifacts["artifacts/query/query1/manifest.json"]) {
		t.Fatalf("artifact payload mismatch")
	}
}

func TestCreateRejectsUnsupportedManifest(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "bad.qdb")
	manifest := Manifest{
		Format:      defaultFormat,
		Version:     "0.0.1",
		ProjectName: "Bad",
		CreatedBy:   "tester",
	}
	err := Create(projectPath, CreateOptions{Manifest: manifest})
	if err == nil {
		t.Fatalf("expected unsupported manifest error")
	}
	if !strings.Contains(err.Error(), "unsupported manifest version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrationContractForLegacyManifestDefaults(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "seed.db")
	if err := os.WriteFile(src, []byte("duckdb"), 0o644); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "old.qdb")
	manifest := Manifest{Version: CurrentSchemaVersion(), ProjectName: "Legacy", CreatedBy: "tester"}
	if err := Create(projectPath, CreateOptions{Manifest: manifest, DatabaseSourcePath: src}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if p.Manifest.Format != defaultFormat || p.Manifest.ArtifactRoot != defaultArtifactRoot || p.Manifest.DataFile != defaultDataFile {
		t.Fatalf("manifest defaults not filled: %#v", p.Manifest)
	}
}

func TestCreateRejectsInvalidArtifactPaths(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "bad-artifacts.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "BadArtifacts"
	manifest.CreatedBy = "tester"

	err := Create(projectPath, CreateOptions{
		Manifest: manifest,
		Artifacts: map[string][]byte{
			"../escape.json": []byte(`{"sql":"bad"}`),
			"/abs/path.json": []byte(`{"sql":"bad"}`),
		},
	})
	if err == nil {
		t.Fatal("expected invalid artifact path error")
	}
	if !strings.Contains(err.Error(), "invalid artifact path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAcceptsArtifactsWithinConfiguredRoot(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "rooted.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "Rooted"
	manifest.CreatedBy = "tester"
	manifest.ArtifactRoot = "project-artifacts/"

	artifacts := map[string][]byte{
		"project-artifacts/query/query1/manifest.json": []byte(`{"id":"query1","kind":"query","schemaVersion":"1.0.0"}`),
	}

	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: dbPath,
		Artifacts:          artifacts,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	if p.Manifest.ArtifactRoot != "project-artifacts/" {
		t.Fatalf("artifact root = %q, want project-artifacts/", p.Manifest.ArtifactRoot)
	}
	if _, err := p.ReadArtifact("project-artifacts/query/query1/manifest.json"); err != nil {
		t.Fatalf("expected artifact to be readable: %v", err)
	}
}

func TestCreateRejectsArtifactsThatEscapeConfiguredRoot(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "root-escape.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "RootEscape"
	manifest.CreatedBy = "tester"
	manifest.ArtifactRoot = "project-artifacts/"

	err := Create(projectPath, CreateOptions{
		Manifest: manifest,
		Artifacts: map[string][]byte{
			"project-artifacts/../leak.json": []byte(`{"sql":"bad"}`),
		},
	})
	if err == nil {
		t.Fatal("expected invalid artifact path error")
	}
	if !strings.Contains(err.Error(), "invalid artifact path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenProjectExposesDataFile(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "payload.duckdb")
	raw, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()
	if _, err := raw.Exec("CREATE TABLE demo(id BIGINT);"); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	projectPath := filepath.Join(tmp, "payload.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "Payload"
	manifest.CreatedBy = "tester"

	if err := Create(projectPath, CreateOptions{Manifest: manifest, DatabaseSourcePath: dbPath}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	data, err := project.ReadDataFile()
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty data file bytes")
	}
}

func TestOpenRejectsMissingManifest(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "missing-manifest.qdb")

	file, err := os.Create(projectPath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	if err := writeZipEntryForTest(zw, defaultDataFile, []byte("duckdb")); err != nil {
		t.Fatalf("write data file: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	if _, err := Open(projectPath); err == nil {
		t.Fatalf("expected missing manifest error")
	} else if !strings.Contains(err.Error(), "manifest entry missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeZipEntryForTest(zw *zip.Writer, path string, data []byte) error {
	entry, err := zw.Create(path)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, bytes.NewReader(data))
	return err
}

func containsString(list []string, value string) bool {
	for _, it := range list {
		if it == value {
			return true
		}
	}
	return false
}
