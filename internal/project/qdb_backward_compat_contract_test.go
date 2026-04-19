package project

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQDBBackwardCompatHandlesLegacyManifestWithoutArtifactRoot(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "legacy.qdb")
	dbPath := filepath.Join(tmp, "legacy.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}

	legacy := Manifest{
		Format:      defaultFormat,
		Version:     CurrentSchemaVersion(),
		ProjectName: "LegacyCompat",
		CreatedBy:   "tester",
	}
	legacyManifest, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy manifest: %v", err)
	}

	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read legacy db bytes: %v", err)
	}

	zfile, err := os.Create(projectPath)
	if err != nil {
		t.Fatalf("create legacy qdb: %v", err)
	}
	defer zfile.Close()

	zw := zip.NewWriter(zfile)
	if err := writeTestZipEntry(zw, manifestEntry, legacyManifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestZipEntry(zw, defaultDataFile, raw); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := writeTestZipEntry(zw, ArtifactManifestPath(defaultArtifactRoot, ArtifactKindCanvas, "legacy-canvas"), []byte(`{"legacy":true}`)); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close legacy qdb: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open legacy project: %v", err)
	}
	if project.Manifest.ArtifactRoot != defaultArtifactRoot {
		t.Fatalf("artifact root = %q, want %q", project.Manifest.ArtifactRoot, defaultArtifactRoot)
	}
	if project.Manifest.DataFile != defaultDataFile {
		t.Fatalf("data file = %q, want %q", project.Manifest.DataFile, defaultDataFile)
	}
	if _, err := project.ReadArtifact(ArtifactManifestPath(project.Manifest.ArtifactRoot, ArtifactKindCanvas, "legacy-canvas")); err != nil {
		t.Fatalf("read artifact: %v", err)
	}
}

func TestQDBBackwardCompatRejectsUnsupportedManifestVersion(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "unsupported.qdb")
	dbPath := filepath.Join(tmp, "db.bin")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write db: %v", err)
	}
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db: %v", err)
	}

	unsupported := Manifest{
		Format:      defaultFormat,
		Version:     "0.9.9",
		ProjectName: "Unsupported",
		CreatedBy:   "tester",
		DataFile:    defaultDataFile,
	}
	manifestData, err := json.Marshal(unsupported)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	file, err := os.Create(projectPath)
	if err != nil {
		t.Fatalf("create qdb: %v", err)
	}
	zw := zip.NewWriter(file)
	if err := writeTestZipEntry(zw, manifestEntry, manifestData); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writeTestZipEntry(zw, defaultDataFile, raw); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close qdb: %v", err)
	}
	file.Close()

	if _, err := Open(projectPath); err == nil {
		t.Fatal("expected unsupported manifest version error")
	} else if !strings.Contains(err.Error(), "unsupported manifest version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeTestZipEntry(zw *zip.Writer, name string, data []byte) error {
	entry, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, bytes.NewReader(data))
	return err
}
