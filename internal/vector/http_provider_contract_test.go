package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHTTPEmbeddingProviderRejectsInvalidConfig(t *testing.T) {
	if _, err := NewHTTPEmbeddingProvider(HTTPEmbeddingProviderConfig{
		Name:      "qwen-local",
		Model:     "qwen3.5-0.8b",
		Endpoint:  "://bad",
		Dimension: 2,
	}); err == nil {
		t.Fatal("expected invalid endpoint error")
	}

	if _, err := NewHTTPEmbeddingProvider(HTTPEmbeddingProviderConfig{
		Name:      "qwen-local",
		Model:     "qwen3.5-0.8b",
		Endpoint:  "http://localhost:0",
		Dimension: 0,
	}); err == nil {
		t.Fatal("expected dimension validation")
	}
}

func TestHTTPEmbeddingProviderRejectsMissingValues(t *testing.T) {
	cases := []HTTPEmbeddingProviderConfig{
		{Name: "", Model: "qwen3.5-0.8b", Endpoint: "http://example.com", Dimension: 2},
		{Name: "qwen", Model: "", Endpoint: "http://example.com", Dimension: 2},
		{Name: "qwen", Model: "qwen3.5-0.8b", Endpoint: "", Dimension: 2},
		{Name: "qwen", Model: "qwen3.5-0.8b", Endpoint: "http://example.com", Dimension: 0},
	}
	for i, cfg := range cases {
		if _, err := NewHTTPEmbeddingProvider(cfg); err == nil {
			t.Fatalf("case %d expected error", i)
		}
	}
}

func TestHTTPEmbeddingProviderEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %s, want /v1/embeddings", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}

		var payload struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "qwen3.5-0.8b" {
			t.Fatalf("model = %q, want qwen3.5-0.8b", payload.Model)
		}
		if len(payload.Input) != 2 || payload.Input[0] != "hello" || payload.Input[1] != "world" {
			t.Fatalf("input = %#v, want [hello world]", payload.Input)
		}

		response := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float64{0.1, 0.2}},
				{"index": 1, "embedding": []float64{0.3, 0.4}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	provider, err := NewHTTPEmbeddingProvider(HTTPEmbeddingProviderConfig{
		Name:      "qwen-local",
		Model:     "qwen3.5-0.8b",
		Endpoint:  server.URL + "/v1/embeddings",
		Dimension: 2,
		APIKey:    "test-token",
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	embeddings, err := provider.Embeddings(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("embeddings: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("embedding count = %d, want 2", len(embeddings))
	}
	if embeddings[0][0] != 0.1 || embeddings[1][1] != 0.4 {
		t.Fatalf("embeddings = %#v", embeddings)
	}
}

func TestHTTPEmbeddingProviderEnforcesDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]any{
			"data": []map[string]any{
				{"index": 0, "embedding": []float64{0.1}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	provider, err := NewHTTPEmbeddingProvider(HTTPEmbeddingProviderConfig{
		Name:      "qwen-local",
		Model:     "qwen3.5-0.8b",
		Endpoint:  server.URL + "/v1/embeddings",
		Dimension: 2,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := provider.Embeddings(context.Background(), []string{"hello"}); err == nil {
		t.Fatal("expected dimension mismatch error")
	}
}

func TestHTTPEmbeddingProviderReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, `{"error":"bad request"}`)
	}))
	defer server.Close()

	provider, err := NewHTTPEmbeddingProvider(HTTPEmbeddingProviderConfig{
		Name:      "qwen-local",
		Model:     "qwen3.5-0.8b",
		Endpoint:  server.URL + "/v1/embeddings",
		Dimension: 2,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if _, err := provider.Embeddings(context.Background(), []string{"bad"}); err == nil {
		t.Fatal("expected provider request failure")
	}
}
