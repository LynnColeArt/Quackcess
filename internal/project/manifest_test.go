package project

import (
	"testing"
)

func TestManifestValidationRequiredFields(t *testing.T) {
	m := Manifest{}
	if err := ValidateManifest(m); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestManifestCurrentVersion(t *testing.T) {
	if got, want := CurrentSchemaVersion(), "1.0.0"; got != want {
		t.Fatalf("schema version = %s, want %s", got, want)
	}
}

func TestManifestValidationSuccess(t *testing.T) {
	m := Manifest{
		Format:      "quackcess.qdb",
		Version:     CurrentSchemaVersion(),
		ProjectName: "demo",
		DataFile:    "data.duckdb",
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
