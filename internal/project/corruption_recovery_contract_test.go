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

func TestCreateDoesNotCorruptExistingProjectOnFailure(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "seed.db")
	if err := os.WriteFile(src, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write seed db: %v", err)
	}

	projectPath := filepath.Join(tmp, "recover.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "Recover"
	manifest.CreatedBy = "tester"
	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: src,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	before, err := os.Stat(projectPath)
	if err != nil {
		t.Fatalf("stat existing project: %v", err)
	}

	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: filepath.Join(tmp, "missing.db"),
	}); err == nil {
		t.Fatal("expected create failure")
	}

	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open existing project after failed create: %v", err)
	}
	after, err := os.Stat(projectPath)
	if err != nil {
		t.Fatalf("stat project after failed create: %v", err)
	}
	if after.Size() != before.Size() {
		t.Fatalf("project size changed from %d to %d", before.Size(), after.Size())
	}

	data, err := p.ReadDataFile()
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected preserved data in corrupted project")
	}
}

func TestOpenRejectsMissingDataFile(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "missing-data.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "MissingData"
	manifest.CreatedBy = "tester"
	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: "",
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	out, err := os.Create(projectPath)
	if err != nil {
		t.Fatalf("overwrite project for corruption test: %v", err)
	}
	zw := zip.NewWriter(out)
	manifestData, err := json.MarshalIndent(p.Manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest for corruption test: %v", err)
	}
	if err := writeZipEntryToWriter(zw, manifestEntry, bytes.NewReader(manifestData)); err != nil {
		t.Fatalf("write manifest-only archive: %v", err)
	}
	if err := zw.Close(); err != nil {
		_ = out.Close()
		t.Fatalf("close corrupt archive: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	if _, err := Open(projectPath); err == nil {
		t.Fatal("expected open to reject missing data file")
	} else if !strings.Contains(err.Error(), "required data file missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertDoesNotMutateProjectStateOnInvalidArtifactSpec(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "upsert-failure.qdb")
	dbPath := filepath.Join(tmp, "seed.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write seed db: %v", err)
	}

	manifest := DefaultManifest()
	manifest.ProjectName = "UpsertFailure"
	manifest.CreatedBy = "tester"
	chartPath := ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "recovery-chart")
	if err := Create(projectPath, CreateOptions{
		Manifest:           manifest,
		DatabaseSourcePath: dbPath,
		Artifacts: map[string][]byte{
			chartPath: []byte(`{"id":"recovery-chart","kind":"chart","schemaVersion":"1.0.0","renderer":"mermaid","chartType":"line","sourceType":"query","source":"SELECT 1"}`),
		},
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}
	original, err := p.ReadArtifact(chartPath)
	if err != nil {
		t.Fatalf("read original artifact: %v", err)
	}

	err = p.UpsertArtifact([]byte(`{bad-json`))
	if err == nil {
		t.Fatal("expected upsert failure")
	}

	rechecked, err := p.ReadArtifact(chartPath)
	if err != nil {
		t.Fatalf("read artifact after failed upsert: %v", err)
	}
	if string(rechecked) != string(original) {
		t.Fatalf("artifact mutated after failed upsert")
	}
}

func writeZipEntryToWriter(zw *zip.Writer, entryPath string, payload *bytes.Reader) error {
	entry, err := zw.Create(entryPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, payload)
	return err
}
