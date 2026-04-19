package vector

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEmbeddingProviderRegistryRegistersAndResolves(t *testing.T) {
	registry := NewEmbeddingProviderRegistry()
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", &namedProvider{
		name:      "qwen-local",
		dimension: 4,
		vectors:   [][]float64{},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	resolved, err := registry.Resolve("qwen-local", "qwen3.5-0.8b")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Name() != "qwen-local" {
		t.Fatalf("resolved provider = %q, want qwen-local", resolved.Name())
	}
}

func TestEmbeddingProviderRegistryRejectsDuplicateRegistration(t *testing.T) {
	registry := NewEmbeddingProviderRegistry()
	provider := &namedProvider{
		name:      "qwen-local",
		dimension: 4,
		vectors:   [][]float64{},
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err != nil {
		t.Fatalf("register first: %v", err)
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestVectorBuildServiceSkipsFreshFieldWhenNotStale(t *testing.T) {
	registry := NewEmbeddingProviderRegistry()
	service := NewVectorBuildService(registry)

	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	service.WithNow(func() time.Time {
		return now
	})

	field := VectorField{
		ID:              "vf-1",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       2,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
		LastIndexedAt:   now,
	}
	result, err := service.BuildFromSourceTexts(
		context.Background(),
		field,
		map[string]string{"a": "first", "b": "second"},
		time.Time{},
		false,
		10,
	)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if result.Built {
		t.Fatal("expected build to be skipped")
	}
	if result.SkipReason == "" {
		t.Fatal("expected skip reason")
	}
}

func TestVectorBuildServiceBuildsWhenForced(t *testing.T) {
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
		t.Fatalf("register: %v", err)
	}
	service := NewVectorBuildService(registry)
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	service.WithNow(func() time.Time {
		return now
	})

	field := VectorField{
		ID:              "vf-2",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       2,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
		LastIndexedAt:   now,
	}
	result, err := service.BuildFromSourceTexts(
		context.Background(),
		field,
		map[string]string{"b": "two", "a": "one", "c": "three"},
		time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC),
		true,
		2,
	)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !result.Built {
		t.Fatal("expected build")
	}
	if result.BatchSize != 2 {
		t.Fatalf("batch size = %d, want 2", result.BatchSize)
	}
	if len(result.VectorsByID) != 3 {
		t.Fatalf("vector count = %d, want 3", len(result.VectorsByID))
	}
	expectedByID := map[string][]float64{
		"a": provider.vectors[0],
		"b": provider.vectors[1],
		"c": provider.vectors[2],
	}
	for id, expected := range expectedByID {
		if !reflect.DeepEqual(result.VectorsByID[id], expected) {
			t.Fatalf("vector for %s = %#v, want %#v", id, result.VectorsByID[id], expected)
		}
	}
	if !result.Field.LastIndexedAt.Equal(now) {
		t.Fatalf("last indexed at = %s, want %s", result.Field.LastIndexedAt, now)
	}
	if len(provider.calls) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.calls))
	}
	if len(provider.calls[0]) != 2 || len(provider.calls[1]) != 1 {
		t.Fatalf("batch sizes = %d/%d, want 2/1", len(provider.calls[0]), len(provider.calls[1]))
	}
	if provider.calls[0][0] != "one" || provider.calls[0][1] != "two" {
		t.Fatalf("first batch order = %#v, want [one two]", provider.calls[0])
	}
}

func TestVectorBuildServicePublishesProgressAndDoneEvents(t *testing.T) {
	registry := NewEmbeddingProviderRegistry()
	provider := &recordingProvider{
		name:      "qwen-local",
		dimension: 2,
		vectors: [][]float64{
			{0.1, 0.2},
			{0.2, 0.3},
			{0.3, 0.4},
			{0.4, 0.5},
			{0.5, 0.6},
		},
	}
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", provider); err != nil {
		t.Fatalf("register: %v", err)
	}
	service := NewVectorBuildService(registry)
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	service.WithNow(func() time.Time {
		return now
	})

	field := VectorField{
		ID:              "vf-progress",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       2,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
	}

	progress := make([]VectorBuildProgress, 0, 3)
	result, err := service.BuildFromSourceTexts(
		context.Background(),
		field,
		map[string]string{
			"row-2": "two",
			"row-1": "one",
			"row-3": "three",
			"row-5": "five",
			"row-4": "four",
		},
		time.Time{},
		true,
		2,
		func(update VectorBuildProgress) {
			progress = append(progress, update)
		},
	)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !result.Built {
		t.Fatal("expected build")
	}
	if len(progress) != 4 {
		t.Fatalf("progress events = %d, want 4", len(progress))
	}
	if progress[0].Done || progress[1].Done || progress[2].Done {
		t.Fatal("expected done flag only on final event")
	}
	if got, want := progress[0].BatchIndex, 1; got != want {
		t.Fatalf("batch index[0] = %d, want %d", got, want)
	}
	if got, want := progress[1].BatchIndex, 2; got != want {
		t.Fatalf("batch index[1] = %d, want %d", got, want)
	}
	if got, want := progress[2].BatchIndex, 3; got != want {
		t.Fatalf("batch index[2] = %d, want %d", got, want)
	}
	if got, want := progress[3].Processed, 5; got != want {
		t.Fatalf("processed[done] = %d, want %d", got, want)
	}
	if !progress[3].Done {
		t.Fatal("expected final event marked done")
	}
	if progress[3].FieldID != "vf-progress" {
		t.Fatalf("fieldId = %q, want vf-progress", progress[3].FieldID)
	}
}

func TestVectorBuildServiceRejectsDimensionMismatch(t *testing.T) {
	registry := NewEmbeddingProviderRegistry()
	if err := registry.Register("qwen-local", "qwen3.5-0.8b", &namedProvider{
		name:      "qwen-local",
		dimension: 4,
		vectors:   [][]float64{{1, 2, 3, 4}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	service := NewVectorBuildService(registry)
	field := VectorField{
		ID:              "vf-3",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       2,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
	}
	if _, err := service.BuildFromSourceTexts(context.Background(), field, map[string]string{"a": "one"}, time.Time{}, true, 1); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "provider dimension") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVectorBuildServiceShouldBuildRespectsForceAndStaleness(t *testing.T) {
	registry := NewEmbeddingProviderRegistry()
	service := NewVectorBuildService(registry)
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	field := VectorField{
		ID:              "vf-4",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       4,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
		LastIndexedAt:   now,
	}
	if service.ShouldBuild(field, time.Time{}, now, false) {
		t.Fatal("expected not to build when fresh")
	}
	if !service.ShouldBuild(field, time.Time{}, now, true) {
		t.Fatal("expected build when force is true")
	}

	field.LastIndexedAt = now.Add(-25 * time.Hour)
	if !service.ShouldBuild(field, time.Time{}, now, false) {
		t.Fatal("expected build for stale index")
	}
}

type namedProvider struct {
	name      string
	dimension int
	vectors   [][]float64
	calls     [][]string
}

func (p *namedProvider) Name() string {
	return p.name
}

func (p *namedProvider) Dimension() int {
	return p.dimension
}

func (p *namedProvider) Embeddings(_ context.Context, inputs []string) ([][]float64, error) {
	p.calls = append(p.calls, append([]string(nil), inputs...))
	output := make([][]float64, 0, len(inputs))
	for range inputs {
		output = append(output, []float64{float64(len(output)), float64(len(output) + 1)})
	}
	return output, nil
}

type recordingProvider struct {
	name      string
	dimension int
	vectors   [][]float64
	calls     [][]string
	cursor    int
}

func (p *recordingProvider) Name() string {
	return p.name
}

func (p *recordingProvider) Dimension() int {
	return p.dimension
}

func (p *recordingProvider) Embeddings(_ context.Context, inputs []string) ([][]float64, error) {
	p.calls = append(p.calls, append([]string(nil), inputs...))
	if p.cursor+len(inputs) > len(p.vectors) {
		return nil, fmt.Errorf("not enough vectors configured")
	}
	output := make([][]float64, 0, len(inputs))
	for i := 0; i < len(inputs); i++ {
		output = append(output, append([]float64(nil), p.vectors[p.cursor+i]...))
	}
	if len(output) != len(inputs) {
		return nil, fmt.Errorf("provider outputs mismatch: %d for %d inputs", len(output), len(inputs))
	}
	p.cursor += len(inputs)
	return output, nil
}
