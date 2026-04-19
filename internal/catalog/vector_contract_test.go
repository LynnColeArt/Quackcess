package catalog

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/vector"
)

func TestVectorFieldRepositoryCreateAndGet(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectors.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repository := NewVectorFieldRepository(database.SQL)
	field := vector.VectorField{
		ID:              "vf-1",
		TableName:       "docs",
		SourceColumn:    "body",
		Dimension:       512,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
		LastIndexedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := repository.Create(field); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repository.GetByID("vf-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TableName != "docs" {
		t.Fatalf("table = %q, want docs", got.TableName)
	}
	if got.VectorColumn != "docs_body_vec" {
		t.Fatalf("vector column = %q, want docs_body_vec", got.VectorColumn)
	}
	if got.SchemaVersion != vector.CurrentVectorFieldSchemaVersion() {
		t.Fatalf("schemaVersion = %q, want %q", got.SchemaVersion, vector.CurrentVectorFieldSchemaVersion())
	}
}

func TestVectorFieldRepositoryUpsertAndList(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectors-upsert.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repository := NewVectorFieldRepository(database.SQL)
	if err := repository.Upsert(vector.VectorField{
		ID:              "vf-1",
		TableName:       "docs",
		SourceColumn:    "body",
		Dimension:       256,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 12,
	}); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if err := repository.Upsert(vector.VectorField{
		ID:              "vf-1",
		TableName:       "docs",
		SourceColumn:    "body",
		Dimension:       512,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 12,
	}); err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	list, err := repository.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Dimension != 512 {
		t.Fatalf("dimension = %d, want %d", list[0].Dimension, 512)
	}
}

func TestVectorFieldRepositoryDelete(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectors-delete.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repository := NewVectorFieldRepository(database.SQL)
	if err := repository.Create(vector.VectorField{
		ID:           "vf-delete",
		TableName:    "docs",
		SourceColumn: "body",
		Dimension:    512,
		Provider:     "qwen-local",
		Model:        "qwen3.5-0.8b",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repository.Delete("vf-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repository.GetByID("vf-delete"); err == nil {
		t.Fatal("expected missing vector field")
	}
}
