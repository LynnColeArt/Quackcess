package vector

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestVectorRebuildServiceStoresVectorsAndMetadata(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "rebuild-vector.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE docs (
		id INTEGER,
		body TEXT,
		body_vec FLOAT[],
		updated_at TIMESTAMP
	);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO docs(id, body, body_vec, updated_at) VALUES
		(1, 'first', NULL, '2026-04-11 09:00:00'),
		(2, 'second', NULL, '2026-04-11 09:15:00');`); err != nil {
		t.Fatalf("seed table: %v", err)
	}

	registry := NewEmbeddingProviderRegistry()
	provider := &recordingProvider{
		name:      "qwen-local",
		dimension: 2,
		vectors: [][]float64{
			{0.1, 0.2},
			{0.2, 0.3},
		},
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	buildService := NewVectorBuildService(registry)
	buildService.WithNow(func() time.Time {
		return time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	})

	repository := &fakeVectorFieldRepository{
		fields: map[string]VectorField{
			"vf-docs": {
				ID:              "vf-docs",
				TableName:       "docs",
				SourceColumn:    "body",
				VectorColumn:    "body_vec",
				Dimension:       2,
				Provider:        "qwen-local",
				Model:           "qwen3.5-0.8b",
				StaleAfterHours: 24,
			},
		},
	}

	service := NewVectorRebuildService(database.SQL, repository, buildService)
	result, err := service.RebuildVector("vf-docs", true)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !result.Built {
		t.Fatalf("result.Built = %v, want true", result.Built)
	}
	if len(result.VectorsByID) != 2 {
		t.Fatalf("vector count = %d, want 2", len(result.VectorsByID))
	}

	var nonNull int
	row := database.SQL.QueryRow(`SELECT COUNT(*) FROM docs WHERE body_vec IS NOT NULL`)
	if err := row.Scan(&nonNull); err != nil {
		t.Fatalf("count vectors: %v", err)
	}
	if nonNull != 2 {
		t.Fatalf("non-null vectors = %d, want 2", nonNull)
	}

	stored, err := repository.GetByID("vf-docs")
	if err != nil {
		t.Fatalf("get from repository: %v", err)
	}
	if stored.LastIndexedAt.IsZero() {
		t.Fatal("stored last indexed at is zero, expected timestamp")
	}
	if got, want := stored.SourceLastUpdatedAt.Format("2006-01-02 15:04:05"), "2026-04-11 09:15:00"; got != want {
		t.Fatalf("stored source last updated at = %s, want %s", got, want)
	}
	if repository.updateCalls != 1 {
		t.Fatalf("repository update calls = %d, want 1", repository.updateCalls)
	}
}

func TestVectorRebuildServiceReportsMissingField(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "rebuild-missing.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	buildService := NewVectorBuildService(NewEmbeddingProviderRegistry())
	service := NewVectorRebuildService(database.SQL, &fakeVectorFieldRepository{fields: map[string]VectorField{}}, buildService)
	if _, err := service.RebuildVector("missing", true); err == nil {
		t.Fatal("expected error")
	} else if err.Error() != "vector field not found: missing" {
		t.Fatalf("error = %q, want %q", err.Error(), "vector field not found: missing")
	}
}

func TestVectorRebuildServicePublishesProgressEvents(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "rebuild-progress.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE docs (
		id INTEGER,
		body TEXT,
		body_vec FLOAT[],
		updated_at TIMESTAMP
	);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO docs(id, body, body_vec, updated_at) VALUES
		(1, 'alpha', NULL, '2026-04-11 09:00:00'),
		(2, 'beta', NULL, '2026-04-11 09:15:00'),
		(3, 'gamma', NULL, '2026-04-11 09:30:00');`); err != nil {
		t.Fatalf("seed table: %v", err)
	}

	registry := NewEmbeddingProviderRegistry()
	provider := &recordingProvider{
		name:      "qwen-local",
		dimension: 2,
		vectors: [][]float64{
			{0.1, 0.2},
			{0.2, 0.3},
			{0.3, 0.4},
		},
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	buildService := NewVectorBuildService(registry)
	buildService.WithNow(func() time.Time {
		return time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	})

	repository := &fakeVectorFieldRepository{
		fields: map[string]VectorField{
			"vf-docs": {
				ID:              "vf-docs",
				TableName:       "docs",
				SourceColumn:    "body",
				VectorColumn:    "body_vec",
				Dimension:       2,
				Provider:        "qwen-local",
				Model:           "qwen3.5-0.8b",
				StaleAfterHours: 24,
			},
		},
	}

	service := NewVectorRebuildService(database.SQL, repository, buildService)
	events := make([]VectorBuildProgress, 0, 3)
	result, err := service.RebuildVectorWithProgress("vf-docs", true, func(update VectorBuildProgress) {
		events = append(events, update)
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !result.Built {
		t.Fatalf("result.Built = %v, want true", result.Built)
	}
	if len(events) != 2 {
		t.Fatalf("progress events = %d, want 2", len(events))
	}
	if events[0].Done {
		t.Fatal("expected initial progress event to not be done")
	}
	if !events[1].Done {
		t.Fatal("expected final progress event to be done")
	}
	if events[1].Processed != 3 {
		t.Fatalf("processed = %d, want 3", events[1].Processed)
	}
}

func TestVectorRebuildServiceFiltersByWhereClause(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "rebuild-filter.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE docs (
		id INTEGER,
		body TEXT,
		body_vec FLOAT[],
		updated_at TIMESTAMP
	);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO docs(id, body, body_vec, updated_at) VALUES
		(1, 'first', NULL, '2026-04-11 09:00:00'),
		(2, 'second', NULL, '2026-04-11 09:15:00'),
		(3, 'third', NULL, '2026-04-11 09:30:00');`); err != nil {
		t.Fatalf("seed table: %v", err)
	}

	registry := NewEmbeddingProviderRegistry()
	provider := &recordingProvider{
		name:      "qwen-local",
		dimension: 2,
		vectors: [][]float64{
			{0.1, 0.2},
			{0.2, 0.3},
			{0.3, 0.4},
		},
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	buildService := NewVectorBuildService(registry)

	repository := &fakeVectorFieldRepository{
		fields: map[string]VectorField{
			"vf-docs": {
				ID:              "vf-docs",
				TableName:       "docs",
				SourceColumn:    "body",
				VectorColumn:    "body_vec",
				Dimension:       2,
				Provider:        "qwen-local",
				Model:           "qwen3.5-0.8b",
				StaleAfterHours: 24,
			},
		},
	}

	service := NewVectorRebuildService(database.SQL, repository, buildService)
	result, err := service.RebuildVectorWithFilter("vf-docs", `id <= 2`, true)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !result.Built {
		t.Fatalf("result.Built = %v, want true", result.Built)
	}
	if len(result.VectorsByID) != 2 {
		t.Fatalf("vector count = %d, want 2", len(result.VectorsByID))
	}

	var nonNull int
	row := database.SQL.QueryRow(`SELECT COUNT(*) FROM docs WHERE body_vec IS NOT NULL`)
	if err := row.Scan(&nonNull); err != nil {
		t.Fatalf("count vectors: %v", err)
	}
	if nonNull != 2 {
		t.Fatalf("non-null vectors = %d, want 2", nonNull)
	}

	var matchedCount int
	row = database.SQL.QueryRow(`SELECT COUNT(*) FROM docs WHERE id <= 2 AND body_vec IS NOT NULL`)
	if err := row.Scan(&matchedCount); err != nil {
		t.Fatalf("count matched vectors: %v", err)
	}
	if matchedCount != 2 {
		t.Fatalf("matched vector count = %d, want 2", matchedCount)
	}

	var excludedCount int
	row = database.SQL.QueryRow(`SELECT COUNT(*) FROM docs WHERE id > 2 AND body_vec IS NOT NULL`)
	if err := row.Scan(&excludedCount); err != nil {
		t.Fatalf("count excluded vectors: %v", err)
	}
	if excludedCount != 0 {
		t.Fatalf("excluded vector count = %d, want 0", excludedCount)
	}
}

func TestVectorRebuildServiceRequiresBuildService(t *testing.T) {
	buildService := NewVectorBuildService(NewEmbeddingProviderRegistry())
	repository := &fakeVectorFieldRepository{fields: map[string]VectorField{}}
	service := NewVectorRebuildService(nil, repository, buildService)
	if _, err := service.RebuildVector("missing", true); err == nil {
		t.Fatal("expected error")
	}
}

type fakeVectorFieldRepository struct {
	fields      map[string]VectorField
	updateCalls int
}

func (f *fakeVectorFieldRepository) GetByID(id string) (VectorField, error) {
	id = normalizeIdentifier(id)
	field, ok := f.fields[id]
	if !ok {
		return VectorField{}, fmt.Errorf("vector field not found: %s", id)
	}
	return field, nil
}

func (f *fakeVectorFieldRepository) Upsert(field VectorField) error {
	f.updateCalls++
	canonical, err := CanonicalizeVectorField(field)
	if err != nil {
		return err
	}
	if f.fields == nil {
		f.fields = map[string]VectorField{}
	}
	f.fields[normalizeIdentifier(canonical.ID)] = canonical
	return nil
}
