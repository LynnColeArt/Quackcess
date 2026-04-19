package vector

import (
	"context"
	"reflect"
	"testing"
)

func TestNewCPUEmbeddingProviderRejectsInvalidConfig(t *testing.T) {
	if _, err := NewCPUEmbeddingProvider(CPUEmbeddingProviderConfig{
		Name:      "qwen-cpu",
		Model:     "qwen3-embedding-0.6b",
		Dimension: 0,
	}); err == nil {
		t.Fatal("expected invalid dimension error")
	}

	if _, err := NewCPUEmbeddingProvider(CPUEmbeddingProviderConfig{
		Name:      "qwen-cpu",
		Model:     "",
		Dimension: 32,
	}); err == nil {
		t.Fatal("expected missing model error")
	}
}

func TestCPUEmbeddingProviderEmbeddingsAreDeterministic(t *testing.T) {
	provider, err := NewCPUEmbeddingProvider(CPUEmbeddingProviderConfig{
		Name:      "qwen-cpu",
		Model:     "qwen3-embedding-0.6b",
		Dimension: 8,
		Seed:      42,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	first, err := provider.Embeddings(context.Background(), []string{"hello", "hello", "world"})
	if err != nil {
		t.Fatalf("embeddings: %v", err)
	}
	second, err := provider.Embeddings(context.Background(), []string{"hello", "hello", "world"})
	if err != nil {
		t.Fatalf("embeddings second: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatal("expected deterministic outputs")
	}
	if len(first) != 3 {
		t.Fatalf("vector count = %d, want 3", len(first))
	}
	if len(first[0]) != 8 || len(first[1]) != 8 || len(first[2]) != 8 {
		t.Fatal("expected all vectors at configured dimension")
	}
	if reflect.DeepEqual(first[0], first[1]) != true {
		t.Fatal("expected repeated input rows to match")
	}
}

func TestCPUEmbeddingProviderReturnsContextError(t *testing.T) {
	provider, err := NewCPUEmbeddingProvider(CPUEmbeddingProviderConfig{
		Name:      "qwen-cpu",
		Model:     "qwen3-embedding-0.6b",
		Dimension: 4,
		Seed:      1,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Embeddings(ctx, []string{"hello"}); err == nil {
		t.Fatal("expected canceled context error")
	}
}
