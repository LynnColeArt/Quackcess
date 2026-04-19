package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectUpsertArtifactAddsAndOverwritesVectorOperationManifest(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "vector-upsert.qdb")
	manifest := DefaultManifest()
	manifest.ProjectName = "VectorUpsert"
	manifest.CreatedBy = "tester"

	if err := Create(projectPath, CreateOptions{Manifest: manifest, DatabaseSourcePath: dbPath}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	initial := VectorOperationSpec{
		ArtifactSpecV1: ArtifactSpecV1{
			ID:            "vector-op-docs-body",
			Kind:          ArtifactKindVectorOp,
			SchemaVersion: CurrentArtifactSchemaVersion(),
		},
		SourceTable:  "docs",
		SourceColumn: "body",
		TargetColumn: "body_vec",
		FieldID:      "vf-docs-body",
		Built:        true,
		BatchSize:    64,
		VectorCount:  10,
		Filter:       "id <= 2",
		CommandText:  "UPDATE docs VECTORIZE body AS body_vec WHERE id <= 2",
		ExecutedAt:   "2025-01-01T00:00:00Z",
		ExecutedBy:   "terminal",
	}
	initialPayload, err := MarshalVectorOperationSpec(initial)
	if err != nil {
		t.Fatalf("marshal initial spec: %v", err)
	}

	if err := p.UpsertArtifact(initialPayload); err != nil {
		t.Fatalf("upsert initial: %v", err)
	}

	contents, err := p.Contents()
	if err != nil {
		t.Fatalf("contents: %v", err)
	}
	manifestPath := ArtifactManifestPath(p.Manifest.ArtifactRoot, ArtifactKindVectorOp, "vector-op-docs-body")
	if !containsString(contents, manifestPath) {
		t.Fatalf("contents missing %q", manifestPath)
	}

	raw, err := p.ReadArtifact(manifestPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	parsed, err := ParseVectorOperationArtifactSpec(raw)
	if err != nil {
		t.Fatalf("parse initial artifact: %v", err)
	}
	if parsed.VectorCount != 10 {
		t.Fatalf("vector count = %d, want 10", parsed.VectorCount)
	}

	updated := initial
	updated.VectorCount = 12
	updated.SkipReason = "not stale"
	updatedPayload, err := MarshalVectorOperationSpec(updated)
	if err != nil {
		t.Fatalf("marshal updated spec: %v", err)
	}
	if err := p.UpsertArtifact(updatedPayload); err != nil {
		t.Fatalf("upsert updated: %v", err)
	}

	raw, err = p.ReadArtifact(manifestPath)
	if err != nil {
		t.Fatalf("read updated artifact: %v", err)
	}
	parsed, err = ParseVectorOperationArtifactSpec(raw)
	if err != nil {
		t.Fatalf("parse updated artifact: %v", err)
	}
	if parsed.VectorCount != 12 {
		t.Fatalf("vector count = %d, want 12", parsed.VectorCount)
	}
	if parsed.SkipReason != "not stale" {
		t.Fatalf("skip reason = %q, want not stale", parsed.SkipReason)
	}
}

func TestProjectUpsertArtifactRejectsInvalidArtifactSpec(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	if err := os.WriteFile(dbPath, []byte("duckdb-placeholder"), 0o644); err != nil {
		t.Fatalf("write db placeholder: %v", err)
	}

	projectPath := filepath.Join(tmp, "vector-upsert-invalid.qdb")
	if err := Create(projectPath, CreateOptions{Manifest: DefaultManifest(), DatabaseSourcePath: dbPath}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	p, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	if err := p.UpsertArtifact([]byte(`{bad-json}`)); err == nil {
		t.Fatal("expected upsert parse error")
	}

	err = p.UpsertArtifact([]byte(`{"id":"","kind":"vector_operation","schemaVersion":"1.0.0","sourceTable":"docs","sourceColumn":"body","targetColumn":"body_vec","fieldId":"vf","commandText":"UPDATE docs VECTORIZE body AS body_vec","executedAt":"2025-01-01T00:00:00Z"}`))
	if err == nil {
		t.Fatal("expected malformed artifact rejection")
	}
	if err.Error() != "artifact id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}
