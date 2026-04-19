package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/vector"
)

type fakeVectorService struct {
	vectorFields  []vector.VectorField
	listErr       error
	rebuildErr    error
	rebuildCalls  int
	rebuildDone   bool
	rebuildResult vector.VectorBuildResult
	searchErr     error
	searchCalls   int
	searchFieldID string
	searchQuery   string
	searchLimit   int
	searchResult  vector.VectorSearchResult
}

func (f *fakeVectorService) ListVectorFields() ([]vector.VectorField, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.vectorFields, nil
}

func (f *fakeVectorService) RebuildVector(fieldID string, force bool) (vector.VectorBuildResult, error) {
	f.rebuildCalls++
	f.rebuildDone = true
	if f.rebuildErr != nil {
		return vector.VectorBuildResult{}, f.rebuildErr
	}
	return f.rebuildResult, nil
}

func (f *fakeVectorService) RebuildVectorWithProgress(
	fieldID string,
	force bool,
	progressCallbacks ...vector.VectorBuildProgressHandler,
) (vector.VectorBuildResult, error) {
	f.rebuildCalls++
	f.rebuildDone = true
	if len(progressCallbacks) > 0 {
		progressCallbacks[0](vector.VectorBuildProgress{FieldID: fieldID, BatchIndex: 1, TotalBatches: 1, BatchSize: 1, Processed: 1, Total: 1, Done: false})
		progressCallbacks[0](vector.VectorBuildProgress{FieldID: fieldID, BatchIndex: 1, TotalBatches: 1, BatchSize: 0, Processed: 1, Total: 1, Done: true})
	}
	if f.rebuildErr != nil {
		return vector.VectorBuildResult{}, f.rebuildErr
	}
	return f.rebuildResult, nil
}

func (f *fakeVectorService) SearchVector(fieldID string, queryText string, limit int) (vector.VectorSearchResult, error) {
	f.searchCalls++
	f.searchFieldID = fieldID
	f.searchQuery = queryText
	f.searchLimit = limit
	if f.searchErr != nil {
		return vector.VectorSearchResult{}, f.searchErr
	}
	return f.searchResult, nil
}

func TestCallToolVectorRebuildRequiresFieldID(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Vector: &fakeVectorService{},
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}
	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "vector.rebuild",
		Args: json.RawMessage(`{}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeInvalidArgument {
		t.Fatalf("result error = %#v, want invalid argument", result.Error)
	}
}

func TestCallToolVectorRebuildReturnsBuildSummary(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Vector: &fakeVectorService{
			rebuildResult: vector.VectorBuildResult{
				Field: vector.VectorField{
					ID: "vf-docs",
				},
				Built:     true,
				BatchSize: 64,
				VectorsByID: map[string][]float64{
					"a": {0.1, 0.2},
					"b": {0.3, 0.4},
				},
			},
		},
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "vector.rebuild",
		Args: json.RawMessage(`{"fieldId":"vf-docs","force":true}`),
	})
	if result.Error != nil {
		t.Fatalf("result error: %#v", result.Error)
	}
	data := result.Data.(map[string]any)
	if data["fieldId"] != "vf-docs" {
		t.Fatalf("fieldId = %v, want vf-docs", data["fieldId"])
	}
	if data["built"] != true {
		t.Fatalf("built = %v, want true", data["built"])
	}
	if data["vectorCount"] != 2 {
		t.Fatalf("vectorCount = %v, want 2", data["vectorCount"])
	}
}

func TestCallToolVectorRebuildPublishesProgressEvents(t *testing.T) {
	eventBus := NewEventBus()
	server := NewServer(NewAllowlistAuthorizer(true), eventBus)

	vectorService := &fakeVectorService{
		rebuildResult: vector.VectorBuildResult{
			Field: vector.VectorField{
				ID: "vf-docs",
			},
			Built:     true,
			BatchSize: 64,
			VectorsByID: map[string][]float64{
				"a": {0.1, 0.2},
				"b": {0.3, 0.4},
			},
		},
	}
	if err := RegisterCoreTools(server, CoreTools{
		Vector:   vectorService,
		EventBus: eventBus,
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	ch, cancel := eventBus.Subscribe(8)
	t.Cleanup(cancel)

	result := server.CallTool(context.Background(), &CallRequest{
		Tool:      "vector.rebuild",
		Args:      json.RawMessage(`{"fieldId":"vf-docs","force":true}`),
		Principal: "agent",
	})
	if result.Error != nil {
		t.Fatalf("result error: %#v", result.Error)
	}

	events := collectEventsFrom(ch, t, 4, eventCallStarted, eventVectorRebuildProgress, eventVectorRebuildProgress, eventCallSuccess)
	progress := 0
	for _, event := range events {
		if event.Type != eventVectorRebuildProgress {
			continue
		}
		payload := event.Payload
		if got, ok := payload["fieldId"]; !ok || got != "vf-docs" {
			t.Fatalf("progress fieldId = %v, want vf-docs", got)
		}
		progress++
	}
	if progress != 2 {
		t.Fatalf("progress events = %d, want 2", progress)
	}
}

func TestCallToolVectorListReturnsSortedCanonicalItems(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Vector: &fakeVectorService{
			vectorFields: []vector.VectorField{
				{
					ID:              "vf-B",
					TableName:       "docs",
					SourceColumn:    "body",
					Dimension:       2,
					Provider:        "qwen-local",
					Model:           "qwen3.5-0.8b",
					StaleAfterHours: 12,
				},
				{
					ID:              " vf-A ",
					TableName:       "orders",
					SourceColumn:    "title",
					Dimension:       2,
					Provider:        "qwen-local",
					Model:           "qwen3.5-0.8b",
					StaleAfterHours: 0,
				},
			},
		},
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{Tool: "vector.list"})
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	items, ok := result.Data.(map[string]any)["items"].([]vector.VectorField)
	if !ok {
		t.Fatalf("items type = %T, want []vector.VectorField", result.Data.(map[string]any)["items"])
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != "vf-A" {
		t.Fatalf("items[0].ID = %q, want vf-A", items[0].ID)
	}
	if items[1].ID != "vf-B" {
		t.Fatalf("items[1].ID = %q, want vf-B", items[1].ID)
	}
	if items[0].StaleAfterHours != 24 {
		t.Fatalf("staleAfterHours = %d, want 24", items[0].StaleAfterHours)
	}
	if items[0].VectorColumn != "orders_title_vec" {
		t.Fatalf("vectorColumn = %q, want orders_title_vec", items[0].VectorColumn)
	}
	// items should be canonicalized and sorted
	if items[1].VectorColumn != "docs_body_vec" {
		t.Fatalf("vectorColumn = %q, want docs_body_vec", items[1].VectorColumn)
	}
}

func TestCallToolVectorListSurfacesServiceError(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Vector: &fakeVectorService{listErr: errors.New("backend unavailable")},
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{Tool: "vector.list"})
	if result.Error == nil || result.Error.Code != ErrorCodeHandlerError {
		t.Fatalf("error = %#v, want handler_error", result.Error)
	}
}

func TestCallToolVectorSearchRequiresQueryAndFieldID(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Vector: &fakeVectorService{},
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "vector.search",
		Args: json.RawMessage(`{}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeInvalidArgument {
		t.Fatalf("result error = %#v, want invalid argument", result.Error)
	}
}

func TestCallToolVectorSearchReturnsMatchesAndHonorsQuery(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	service := &fakeVectorService{
		searchResult: vector.VectorSearchResult{
			Field: vector.VectorField{
				ID:           "vf-docs",
				TableName:    "docs",
				SourceColumn: "body",
				VectorColumn: "body_vec",
			},
			Matches: []vector.SimilarityMatch{
				{ID: "row-1", Score: 0.8},
				{ID: "row-2", Score: 0.7},
			},
		},
	}
	if err := RegisterCoreTools(server, CoreTools{
		Vector: service,
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "vector.search",
		Args: json.RawMessage(`{"fieldId":"vf-docs","query":"find report","limit":3}`),
	})
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	if service.searchCalls != 1 {
		t.Fatalf("searchCalls = %d, want 1", service.searchCalls)
	}
	if service.searchFieldID != "vf-docs" {
		t.Fatalf("fieldId = %q, want vf-docs", service.searchFieldID)
	}
	if service.searchQuery != "find report" {
		t.Fatalf("query = %q, want find report", service.searchQuery)
	}
	if service.searchLimit != 3 {
		t.Fatalf("limit = %d, want 3", service.searchLimit)
	}

	data := result.Data.(map[string]any)
	if data["fieldId"] != "vf-docs" {
		t.Fatalf("fieldId = %v, want vf-docs", data["fieldId"])
	}
	if data["query"] != "find report" {
		t.Fatalf("query = %v, want find report", data["query"])
	}
	if data["vectorCount"] != 2 {
		t.Fatalf("vectorCount = %v, want 2", data["vectorCount"])
	}
	items, ok := data["items"].([]vector.SimilarityMatch)
	if !ok {
		t.Fatalf("items type = %T, want []vector.SimilarityMatch", data["items"])
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if items[0].ID != "row-1" || items[1].ID != "row-2" {
		t.Fatalf("items = %#v, want row-1,row-2", items)
	}
}

func TestCallToolVectorSearchPropagatesServiceError(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Vector: &fakeVectorService{
			searchErr: errors.New("vector index unavailable"),
		},
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "vector.search",
		Args: json.RawMessage(`{"fieldId":"vf-docs","query":"find report"}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeHandlerError {
		t.Fatalf("result error = %#v, want handler_error", result.Error)
	}
}
