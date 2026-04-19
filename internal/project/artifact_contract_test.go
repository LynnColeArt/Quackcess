package project

import (
	"strings"
	"testing"
)

func TestArtifactManifestPathIsDeterministic(t *testing.T) {
	root := "artifacts/"
	pathA := ArtifactManifestPath(root, ArtifactKindCanvas, "canvas-001")
	pathB := ArtifactManifestPath(root, ArtifactKindCanvas, "canvas-001")
	if pathA != pathB {
		t.Fatalf("path = %q, want same deterministic path as %q", pathA, pathB)
	}
}

func TestArtifactManifestPathUsesKindAndIdNamespace(t *testing.T) {
	path := ArtifactManifestPath("artifacts/", ArtifactKindChart, "ch-42")
	if path != "artifacts/chart/ch-42/manifest.json" {
		t.Fatalf("manifest path = %q, want artifacts/chart/ch-42/manifest.json", path)
	}
}

func TestArtifactManifestPathRejectsMissingKindOrId(t *testing.T) {
	if _, err := NewArtifactKind(""); err == nil {
		t.Fatal("expected missing-kind error")
	}
}

func TestArtifactManifestPathRejectsEmptyArtifactId(t *testing.T) {
	if _, err := ArtifactManifestPathValidated(ArtifactKindCanvas, ""); err == nil {
		t.Fatal("expected missing id error")
	}
}

func TestNewArtifactKindSupportsOnlyKnownKinds(t *testing.T) {
	if _, err := NewArtifactKind("dashboard"); err == nil {
		t.Fatal("expected unsupported kind error")
	}
	if _, err := NewArtifactKind(string(ArtifactKindVectorOp)); err != nil {
		t.Fatalf("vector operation kind should be supported: %v", err)
	}
}

func TestValidateArtifactSpecV1RequiresStableFields(t *testing.T) {
	err := ValidateArtifactSpecV1(ArtifactSpecV1{
		ID:            "a1",
		Kind:          ArtifactKindCanvas,
		SchemaVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = ValidateArtifactSpecV1(ArtifactSpecV1{
		ID:            "a1",
		Kind:          ArtifactKindChart,
		SchemaVersion: "",
	})
	if err == nil {
		t.Fatal("expected schema version required error")
	}
}

func TestParseArtifactSpecMigratesLegacyKindField(t *testing.T) {
	spec, err := ParseArtifactSpec([]byte(`{"id":"legacy-canvas","artifactType":"canvas","title":"Legacy"}`))
	if err != nil {
		t.Fatalf("parse legacy spec: %v", err)
	}
	if spec.ID != "legacy-canvas" {
		t.Fatalf("spec id = %q, want legacy-canvas", spec.ID)
	}
	if spec.Kind != ArtifactKindCanvas {
		t.Fatalf("spec kind = %q, want canvas", spec.Kind)
	}
	if spec.SchemaVersion != CurrentArtifactSchemaVersion() {
		t.Fatalf("schemaVersion = %q, want %q", spec.SchemaVersion, CurrentArtifactSchemaVersion())
	}
}

func TestParseArtifactSpecRejectsUnsupportedSchemaVersion(t *testing.T) {
	_, err := ParseArtifactSpec([]byte(`{"id":"bad","kind":"canvas","schemaVersion":"9.9.9"}`))
	if err == nil {
		t.Fatal("expected unsupported schema version error")
	}
	if !strings.Contains(err.Error(), "unsupported artifact schema version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildArtifactIndexDetectsDuplicateIds(t *testing.T) {
	_, err := BuildArtifactIndex(defaultArtifactRoot, map[string][]byte{
		ArtifactManifestPath(defaultArtifactRoot, ArtifactKindCanvas, "shared-id"): []byte(`{"id":"shared-id","kind":"canvas","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(defaultArtifactRoot, ArtifactKindChart, "shared-id"):  []byte(`{"id":"shared-id","kind":"chart","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(defaultArtifactRoot, ArtifactKindQuery, "other-id"):   []byte(`{"id":"other-id","kind":"query","schemaVersion":"1.0.0"}`),
	})
	if err == nil {
		t.Fatal("expected duplicate artifact id error")
	}
	if !strings.Contains(err.Error(), "duplicate artifact id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildArtifactIndexRejectsStaleManifestPath(t *testing.T) {
	_, err := BuildArtifactIndex(defaultArtifactRoot, map[string][]byte{
		"artifacts/query/mismatch/manifest.json": []byte(`{"id":"real-id","kind":"query","schemaVersion":"1.0.0"}`),
	})
	if err == nil {
		t.Fatal("expected path mismatch error")
	}
	if !strings.Contains(err.Error(), "artifact manifest path mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseVectorOperationArtifactSpecSupportsVectorSchema(t *testing.T) {
	raw := []byte(`{
		"id": "vectorize-docs-body-body_vec",
		"kind": "vector_operation",
		"schemaVersion": "1.0.0",
		"sourceTable": "docs",
		"sourceColumn": "body",
		"targetColumn": "body_vec",
		"fieldId": "vf-docs-body",
		"built": true,
		"batchSize": 64,
		"vectorCount": 2,
		"commandText": "UPDATE docs VECTORIZE body AS body_vec",
		"executedAt": "2025-01-01T00:00:00Z"
	}`)

	spec, err := ParseVectorOperationArtifactSpec(raw)
	if err != nil {
		t.Fatalf("parse vector operation spec: %v", err)
	}
	if spec.Kind != ArtifactKindVectorOp {
		t.Fatalf("spec kind = %q, want %q", spec.Kind, ArtifactKindVectorOp)
	}
	if spec.VectorCount != 2 {
		t.Fatalf("vector count = %d, want 2", spec.VectorCount)
	}
}

func TestParseVectorOperationArtifactSpecRejectsWrongKind(t *testing.T) {
	_, err := ParseVectorOperationArtifactSpec([]byte(`{"id":"oops","kind":"query","schemaVersion":"1.0.0","sourceTable":"docs","sourceColumn":"body","targetColumn":"body_vec","fieldId":"vf","commandText":"UPDATE docs VECTORIZE body AS body_vec","executedAt":"2025-01-01T00:00:00Z"}`))
	if err == nil {
		t.Fatal("expected invalid kind error")
	}
	if !strings.Contains(err.Error(), "artifact kind mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}
