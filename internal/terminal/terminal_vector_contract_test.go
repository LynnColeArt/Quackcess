package terminal

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/vector"
)

func TestTerminalVectorListRequiresService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-list-no-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)
	result, err := service.RunCommand("\\vector list")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "vector service is not configured" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "vector service is not configured")
	}
}

func TestTerminalVectorListReturnsCanonicalizedRows(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-list.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{
			{
				ID:           " vf-docs ",
				TableName:    "docs",
				SourceColumn: "body",
				Dimension:    128,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector list")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if result.RowCount != 1 {
		t.Fatalf("row count = %d, want 1", result.RowCount)
	}
	if got := result.Rows[0][0]; got != "vf-docs" {
		t.Fatalf("field id = %#v, want vf-docs", got)
	}
	if got := result.Rows[0][3]; got != "docs_body_vec" {
		t.Fatalf("vector column = %#v, want docs_body_vec", got)
	}
}

func TestTerminalVectorSearchRequiresService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-no-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)
	result, err := service.RunCommand("\\vector search vf-search hello")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "vector service is not configured" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "vector service is not configured")
	}
}

func TestTerminalVectorSearchReturnsMatchesAndCallsService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-results.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		searchResult: vector.VectorSearchResult{
			Field: vector.VectorField{
				ID:           "vf-search",
				TableName:    "docs",
				SourceColumn: "body",
				VectorColumn: "body_vec",
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
			Matches: []vector.SimilarityMatch{
				{ID: "docs-1", Score: 0.95},
				{ID: "docs-3", Score: 0.82},
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector search vf-search dog in the woods --limit 2")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if result.Columns[0] != "id" || result.Columns[1] != "score" {
		t.Fatalf("unexpected columns = %#v", result.Columns)
	}
	if result.RowCount != 2 {
		t.Fatalf("row count = %d, want 2", result.RowCount)
	}
	if result.Rows[0][0] != "docs-1" || result.Rows[0][1] != 0.95 {
		t.Fatalf("match row = %#v, want [docs-1 0.95]", result.Rows[0])
	}
	if resultService.searchCalls != 1 {
		t.Fatalf("search calls = %d, want 1", resultService.searchCalls)
	}
	if resultService.searchFieldID != "vf-search" {
		t.Fatalf("fieldId = %q, want vf-search", resultService.searchFieldID)
	}
	if resultService.searchLimit != 2 {
		t.Fatalf("limit = %d, want 2", resultService.searchLimit)
	}
	if resultService.searchQuery != "dog in the woods" {
		t.Fatalf("query = %q, want dog in the woods", resultService.searchQuery)
	}
}

func TestTerminalVectorSearchRejectsMalformedLimit(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-bad-limit.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		searchResult: vector.VectorSearchResult{
			Matches: []vector.SimilarityMatch{},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector search vf-search find --limit")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vector usage: \\vector search <field-id> <query> [--limit <n>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorSearchDisallowsMissingQuery(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-search-empty-query.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector search vf-search")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vector usage: \\vector search <field-id> <query> [--limit <n>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorListDisallowsExtraArguments(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-list-extra.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector list extra")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vector usage: \\vector list") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorListPropagatesServiceError(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-list-error.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		listErr: fmt.Errorf("vector repo unavailable"),
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector list")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "vector repo unavailable" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "vector repo unavailable")
	}
}

func TestTerminalVectorRebuildReturnsSummaryAndPropagatesForce(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-rebuild.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	rebuildField := vector.VectorField{
		ID:              "vf-rebuild",
		TableName:       "docs",
		SourceColumn:    "body",
		Dimension:       2,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
	}
	resultService := &fakeVectorService{
		rebuildResult: vector.VectorBuildResult{
			Field:      rebuildField,
			Built:      true,
			BatchSize:  2,
			SkipReason: "",
			VectorsByID: map[string][]float64{
				"docs-1": {0.1, 0.2},
				"docs-2": {0.2, 0.3},
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector rebuild vf-rebuild --force")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if result.RowCount != 1 {
		t.Fatalf("row count = %d, want 1", result.RowCount)
	}
	if result.Columns[1] != "status" {
		t.Fatalf("columns[1] = %q, want status", result.Columns[1])
	}
	if result.Rows[0][1] != "built" {
		t.Fatalf("status = %q, want built", result.Rows[0][1])
	}
	if result.Rows[0][2] != 2 {
		t.Fatalf("batch size = %v, want 2", result.Rows[0][2])
	}
	if result.Rows[0][3] != 2 {
		t.Fatalf("vector_count = %v, want 2", result.Rows[0][3])
	}
	if resultService.rebuildCalls == 0 || resultService.rebuildFieldID != "vf-rebuild" {
		t.Fatalf("expected rebuild called for vf-rebuild, got %q (calls=%d)", resultService.rebuildFieldID, resultService.rebuildCalls)
	}
	if !resultService.rebuildForce {
		t.Fatal("expected rebuild force flag to be true")
	}
}

func TestTerminalVectorRebuildRejectsMalformedSyntax(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-rebuild-bad.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		rebuildResult: vector.VectorBuildResult{
			Field: vector.VectorField{
				ID:              "vf-rebuild",
				TableName:       "docs",
				SourceColumn:    "body",
				Dimension:       2,
				Provider:        "qwen-local",
				Model:           "qwen3.5-0.8b",
				StaleAfterHours: 24,
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector rebuild")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vector usage: \\vector rebuild <field-id> [--force]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorRebuildPropagatesServiceError(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-rebuild-error.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		rebuildErr: fmt.Errorf("index engine down"),
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector rebuild vf-rebuild")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "index engine down" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "index engine down")
	}
}

func TestTerminalVectorUnknownSubcommandReturnsUsage(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vector-unknown.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("\\vector unknown")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vector usage: \\vector list | \\vector rebuild <field-id> [--force] | \\vector search <field-id> <query> [--limit <n>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorizeCommandRebuildsTargetedFieldWithFilter(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-target-with-filter.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	rebuildField := vector.VectorField{
		ID:           "vf-docs-body-vec2",
		TableName:    "docs",
		SourceColumn: "body",
		VectorColumn: "body_vec2",
		Dimension:    2,
		Provider:     "qwen-local",
		Model:        "qwen3.5-0.8b",
	}
	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{
			{
				ID:           "vf-docs-body",
				TableName:    "docs",
				SourceColumn: "body",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
			rebuildField,
		},
		rebuildWithFilterResult: vector.VectorBuildResult{
			Field: rebuildField,
			Built: true,
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body AS body_vec2 WHERE id <= 2")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if resultService.rebuildWithFilterCalls != 1 {
		t.Fatalf("rebuild with filter calls = %d, want 1", resultService.rebuildWithFilterCalls)
	}
	if resultService.rebuildWithFilterFieldID != "vf-docs-body-vec2" {
		t.Fatalf("field id = %q, want vf-docs-body-vec2", resultService.rebuildWithFilterFieldID)
	}
	if resultService.rebuildWithFilterFilter != "id <= 2" {
		t.Fatalf("filter = %q, want id <= 2", resultService.rebuildWithFilterFilter)
	}
	if result.Rows[0][6] != "body" {
		t.Fatalf("source column = %v, want body", result.Rows[0][6])
	}
	if result.Rows[0][7] != "body_vec2" {
		t.Fatalf("target column = %v, want body_vec2", result.Rows[0][7])
	}
}

func TestTerminalVectorizeCommandRebuildsTargetedFieldWithoutFilter(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-unique-source.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	rebuildField := vector.VectorField{
		ID:           "vf-docs-body",
		TableName:    "docs",
		SourceColumn: "body",
		VectorColumn: "body_vec",
		Dimension:    2,
		Provider:     "qwen-local",
		Model:        "qwen3.5-0.8b",
	}
	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{rebuildField},
		rebuildResult: vector.VectorBuildResult{
			Field:       rebuildField,
			Built:       true,
			BatchSize:   1,
			VectorsByID: map[string][]float64{"docs-1": {0.4, 0.5}},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body AS body_vec")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if resultService.rebuildCalls != 1 {
		t.Fatalf("rebuild calls = %d, want 1", resultService.rebuildCalls)
	}
	if resultService.rebuildFieldID != "vf-docs-body" {
		t.Fatalf("field id = %q, want vf-docs-body", resultService.rebuildFieldID)
	}
	if resultService.rebuildWithFilterCalls != 0 {
		t.Fatalf("rebuild with filter calls = %d, want 0", resultService.rebuildWithFilterCalls)
	}
	if result.Vectorize == nil {
		t.Fatal("expected vector metadata")
	}
	if result.Vectorize.FieldID != "vf-docs-body" {
		t.Fatalf("metadata field ID = %q, want vf-docs-body", result.Vectorize.FieldID)
	}
	if result.Vectorize.VectorCount != 1 {
		t.Fatalf("metadata vector count = %d, want 1", result.Vectorize.VectorCount)
	}
	if result.Vectorize.Filter != "" {
		t.Fatalf("metadata filter = %q, want empty", result.Vectorize.Filter)
	}
}

func TestTerminalVectorizeCommandSupportsShorthandTargetSyntax(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-shorthand.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	rebuildField := vector.VectorField{
		ID:           "vf-docs-body",
		TableName:    "docs",
		SourceColumn: "body",
		VectorColumn: "body_vec",
		Dimension:    2,
		Provider:     "qwen-local",
		Model:        "qwen3.5-0.8b",
	}
	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{rebuildField},
		rebuildResult: vector.VectorBuildResult{
			Field:       rebuildField,
			Built:       true,
			BatchSize:   1,
			VectorsByID: map[string][]float64{"docs-1": {0.9, 0.1}},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body body_vec WHERE id <= 2")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if resultService.rebuildWithFilterCalls != 1 {
		t.Fatalf("rebuild with filter calls = %d, want 1", resultService.rebuildWithFilterCalls)
	}
	if resultService.rebuildWithFilterFieldID != "vf-docs-body" {
		t.Fatalf("field id = %q, want vf-docs-body", resultService.rebuildWithFilterFieldID)
	}
	if resultService.rebuildWithFilterFilter != "id <= 2" {
		t.Fatalf("filter = %q, want id <= 2", resultService.rebuildWithFilterFilter)
	}
}

func TestTerminalVectorizeCommandRequiresTargetVectorColumn(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-ambiguous-source.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{
			{
				ID:           "vf-docs-body",
				TableName:    "docs",
				SourceColumn: "body",
				VectorColumn: "body_vec",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
			{
				ID:           "vf-docs-body-2",
				TableName:    "docs",
				SourceColumn: "body",
				VectorColumn: "body_vec2",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "multiple vector fields found for docs.body; include target vector column") {
		t.Fatalf("error = %q, expected ambiguity error", result.ErrorText)
	}
}

func TestTerminalVectorizeCommandRebuildsUniqueSourceWithoutTarget(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-target-omitted.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	rebuildField := vector.VectorField{
		ID:           "vf-docs-body",
		TableName:    "docs",
		SourceColumn: "body",
		VectorColumn: "body_vec",
		Dimension:    2,
		Provider:     "qwen-local",
		Model:        "qwen3.5-0.8b",
	}
	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{rebuildField},
		rebuildResult: vector.VectorBuildResult{
			Field:       rebuildField,
			Built:       true,
			BatchSize:   1,
			VectorsByID: map[string][]float64{"docs-1": {0.1, 0.2}},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if resultService.rebuildCalls != 1 {
		t.Fatalf("rebuild calls = %d, want 1", resultService.rebuildCalls)
	}
	if resultService.rebuildFieldID != "vf-docs-body" {
		t.Fatalf("field id = %q, want vf-docs-body", resultService.rebuildFieldID)
	}
	if result.Vectorize == nil || result.Vectorize.FieldID != "vf-docs-body" {
		t.Fatal("expected vector metadata")
	}
}

func TestTerminalVectorizeCommandRebuildsUniqueSourceWithoutTargetWithFilter(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-target-omitted-where.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	rebuildField := vector.VectorField{
		ID:           "vf-docs-body",
		TableName:    "docs",
		SourceColumn: "body",
		VectorColumn: "body_vec",
		Dimension:    2,
		Provider:     "qwen-local",
		Model:        "qwen3.5-0.8b",
	}
	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{rebuildField},
		rebuildWithFilterResult: vector.VectorBuildResult{
			Field: rebuildField,
			Built: true,
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body WHERE id = 1")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if resultService.rebuildWithFilterCalls != 1 {
		t.Fatalf("rebuild with filter calls = %d, want 1", resultService.rebuildWithFilterCalls)
	}
	if resultService.rebuildWithFilterFilter != "id = 1" {
		t.Fatalf("filter = %q, want id = 1", resultService.rebuildWithFilterFilter)
	}
}

func TestTerminalVectorizeCommandRejectsMalformedAsClause(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-bad-as.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{
			{
				ID:           "vf-docs-body",
				TableName:    "docs",
				SourceColumn: "body",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body AS WHERE id = 1")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vectorize usage: UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorizeCommandRejectsMissingTargetAfterAs(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-missing-target.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body AS WHERE id = 1")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vectorize usage: UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorizeCommandRejectsWhereWithoutFilter(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-where-no-filter.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{
			{
				ID:           "vf-docs-body",
				TableName:    "docs",
				SourceColumn: "body",
				VectorColumn: "body_vec",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body AS body_vec WHERE")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vectorize usage: UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

func TestTerminalVectorizeCommandRejectsUnknownTokenAfterTarget(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "vectorize-unknown-token.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	resultService := &fakeVectorService{
		vectorFields: []vector.VectorField{
			{
				ID:           "vf-docs-body",
				TableName:    "docs",
				SourceColumn: "body",
				VectorColumn: "body_vec",
				Dimension:    2,
				Provider:     "qwen-local",
				Model:        "qwen3.5-0.8b",
			},
		},
	}
	service := NewTerminalServiceWithCanvasRepositoryAndVectorService(database.SQL, nil, nil, resultService)

	result, err := service.RunCommand("UPDATE docs VECTORIZE body AS body_vec TO id = 1")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "vectorize usage: UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>]") {
		t.Fatalf("error = %q, expected usage", result.ErrorText)
	}
}

type fakeVectorService struct {
	vectorFields             []vector.VectorField
	listErr                  error
	rebuildErr               error
	rebuildWithFilterErr     error
	rebuildCalls             int
	rebuildFieldID           string
	rebuildForce             bool
	rebuildResult            vector.VectorBuildResult
	rebuildWithFilterCalls   int
	rebuildWithFilterFieldID string
	rebuildWithFilterFilter  string
	rebuildWithFilterForce   bool
	rebuildWithFilterResult  vector.VectorBuildResult
	searchErr                error
	searchCalls              int
	searchFieldID            string
	searchQuery              string
	searchLimit              int
	searchResult             vector.VectorSearchResult
}

func (s *fakeVectorService) ListVectorFields() ([]vector.VectorField, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.vectorFields, nil
}

func (s *fakeVectorService) RebuildVector(fieldID string, force bool) (vector.VectorBuildResult, error) {
	s.rebuildCalls++
	s.rebuildFieldID = fieldID
	s.rebuildForce = force
	if s.rebuildErr != nil {
		return vector.VectorBuildResult{}, s.rebuildErr
	}
	return s.rebuildResult, nil
}

func (s *fakeVectorService) RebuildVectorWithFilter(fieldID string, filter string, force bool) (vector.VectorBuildResult, error) {
	s.rebuildWithFilterCalls++
	s.rebuildWithFilterFieldID = fieldID
	s.rebuildWithFilterFilter = filter
	s.rebuildWithFilterForce = force
	if s.rebuildWithFilterErr != nil {
		return vector.VectorBuildResult{}, s.rebuildWithFilterErr
	}
	return s.rebuildWithFilterResult, nil
}

func (s *fakeVectorService) SearchVector(fieldID string, queryText string, limit int) (vector.VectorSearchResult, error) {
	s.searchCalls++
	s.searchFieldID = fieldID
	s.searchQuery = queryText
	s.searchLimit = limit
	if s.searchErr != nil {
		return vector.VectorSearchResult{}, s.searchErr
	}
	return s.searchResult, nil
}
