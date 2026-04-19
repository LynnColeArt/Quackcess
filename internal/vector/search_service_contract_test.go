package vector

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestVectorSearchServiceReturnsRankedMatches(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE docs (title TEXT, embedding TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO docs(title, embedding) VALUES
		('alpha', '[1,0]'),
		('beta', '[0,1]'),
		('bad', 'oops');`); err != nil {
		t.Fatalf("seed table: %v", err)
	}

	registry := NewEmbeddingProviderRegistry()
	provider := &namedProvider{
		name:      "qwen-local",
		dimension: 2,
		vectors: [][]float64{
			{0, 1},
		},
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	rebuildService := NewVectorBuildService(registry)

	repository := &fakeVectorFieldRepository{
		fields: map[string]VectorField{
			"vf-docs": {
				ID:           "vf-docs",
				TableName:    "docs",
				SourceColumn: "title",
				VectorColumn: "embedding",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewVectorSearchService(database.SQL, repository, rebuildService)

	result, err := service.SearchByFieldID(context.Background(), "vf-docs", "find docs", 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result.Field.ID != "vf-docs" {
		t.Fatalf("field.ID = %q, want vf-docs", result.Field.ID)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(result.Matches))
	}
	if result.Matches[0].ID == "" {
		t.Fatalf("match id should not be empty")
	}
	if result.Matches[0].Score != 1 {
		t.Fatalf("top score = %f, want 1", result.Matches[0].Score)
	}
}

func TestVectorSearchServiceRequiresQueryText(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-service-empty-query.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	registry := NewEmbeddingProviderRegistry()
	service := NewVectorSearchService(database.SQL, &fakeVectorFieldRepository{}, NewVectorBuildService(registry))
	if _, err := service.SearchByFieldID(context.Background(), "vf", "", 5); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "query text is required") {
		t.Fatalf("error = %v, want query text validation", err)
	}
}

func TestVectorSearchServiceRejectsProviderDimensionMismatch(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-service-mismatch.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE docs (title TEXT, embedding TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO docs(title, embedding) VALUES
		('alpha', '[1,2]');`); err != nil {
		t.Fatalf("seed table: %v", err)
	}

	registry := NewEmbeddingProviderRegistry()
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", &namedProvider{
		name:      "qwen-local",
		dimension: 3,
		vectors:   [][]float64{{1, 0, 0}},
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	repository := &fakeVectorFieldRepository{
		fields: map[string]VectorField{
			"vf-docs": {
				ID:           "vf-docs",
				TableName:    "docs",
				SourceColumn: "title",
				VectorColumn: "embedding",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewVectorSearchService(database.SQL, repository, NewVectorBuildService(registry))
	if _, err := service.SearchByFieldID(context.Background(), "vf-docs", "find", 5); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "provider returned vector dimension") {
		t.Fatalf("error = %v, want provider dimension mismatch", err)
	}
}

func TestParseVectorRawValueSupportsAllKnownFormats(t *testing.T) {
	rawValues := map[string]any{
		"floatSlice": []float64{1, 2, 3},
		"jsonString": `[1,2,3]`,
		"byteSlice":  []byte(`[1,2,3]`),
		"jsonArray":  []any{float64(1), float64(2), float64(3)},
	}

	got := parseVectorRawValue(rawValues["floatSlice"])
	if got == nil || len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Fatalf("parse float slice = %#v", got)
	}
	got = parseVectorRawValue(rawValues["jsonString"])
	if got == nil || len(got) != 3 || got[1] != 2 {
		t.Fatalf("parse json string = %#v", got)
	}
	got = parseVectorRawValue(rawValues["byteSlice"])
	if got == nil || len(got) != 3 || got[2] != 3 {
		t.Fatalf("parse []byte = %#v", got)
	}
	got = parseVectorRawValue(rawValues["jsonArray"])
	if got == nil || len(got) != 3 || got[1] != 2 {
		t.Fatalf("parse []any = %#v", got)
	}
	if got := parseVectorRawValue("bad-json"); got != nil {
		t.Fatalf("parse bad json = %#v, want nil", got)
	}
}
