package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRunInstallCPUBackendSkipsRemoteCheck(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "cpu")

	output, err := captureStdout(func() error {
		return run([]string{"install"})
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}

	if strings.TrimSpace(output) != "vector backend ready: cpu" {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestRunInstallVerifiesModelFromModelsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"data":[{"id":"qwen3-embedding-0.6b"},{"id":"other"}]}`)
		default:
			t.Fatalf("unexpected endpoint path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("QUACKCESS_VECTOR_BACKEND", "llama-cpp")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", server.URL+"/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3-embedding-0.6b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "3")

	output, err := captureStdout(func() error {
		return run([]string{"install"})
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}
	if !strings.Contains(output, "vector model ready: backend=llamacpp") {
		t.Fatalf("expected readiness message, got %q", output)
	}
}

func TestRunInstallFallsBackToProbeWhenModelListUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusNotFound)
		case "/v1/api/tags", "/api/tags", "/models", "/api/pull", "/v1/api/pull":
			w.WriteHeader(http.StatusNotFound)
		case "/v1/embeddings":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for embeddings probe: %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}`)
		default:
			t.Fatalf("unexpected endpoint path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("QUACKCESS_VECTOR_BACKEND", "llama-cpp")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", server.URL+"/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3-embedding-0.6b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "3")

	output, err := captureStdout(func() error {
		return run([]string{"install"})
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}
	if !strings.Contains(output, "vector model ready: backend=llamacpp") {
		t.Fatalf("expected readiness message, got %q", output)
	}
}

func TestRunInstallDownloadsModelWhenMissing(t *testing.T) {
	models := map[string]bool{
		"other-model": true,
	}

	var pullCalled bool
	var lock sync.Mutex

	render := func(w http.ResponseWriter) {
		current := make([]map[string]any, 0, len(models))
		for modelName := range models {
			current = append(current, map[string]any{"id": modelName})
		}
		response := map[string]any{"data": current}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode model list response: %v", err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()

		switch r.URL.Path {
		case "/v1/models":
			render(w)
		case "/api/pull", "/v1/api/pull":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for pull: %s", r.Method)
			}
			pullCalled = true
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode pull payload: %v", err)
			}
			models[payload.Name] = true
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/tags", "/v1/api/tags":
			render(w)
		default:
			t.Fatalf("unexpected endpoint path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("QUACKCESS_VECTOR_BACKEND", "http")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", server.URL+"/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3-embedding-0.6b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "3")

	output, err := captureStdout(func() error {
		return run([]string{"install"})
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}
	if !strings.Contains(output, "vector model ready: backend=http") {
		t.Fatalf("expected readiness message, got %q", output)
	}
	if !strings.Contains(output, "vector model not loaded, attempting download") {
		t.Fatalf("expected download attempt output, got %q", output)
	}

	lock.Lock()
	defer lock.Unlock()
	if !pullCalled {
		t.Fatalf("expected model pull endpoint to be called")
	}
}

func TestRunInstallDefaultsToExpectedTinyModelWhenMissing(t *testing.T) {
	models := map[string]bool{
		"other-model": true,
	}
	var pulledModel string
	var pullCalled bool

	var lock sync.Mutex
	renderModels := func() []byte {
		current := make([]map[string]any, 0, len(models))
		for modelName := range models {
			current = append(current, map[string]any{"id": modelName})
		}
		response := map[string]any{"data": current}
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("marshal model list response: %v", err)
		}
		return raw
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()

		switch r.URL.Path {
		case "/v1/models", "/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(renderModels())
		case "/v1/embeddings":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method for probe: %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}`))
		case "/api/pull", "/v1/api/pull":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected pull method: %s", r.Method)
			}
			var payload struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode pull payload: %v", err)
			}
			pullCalled = true
			pulledModel = payload.Name
			models[pulledModel] = true
			_, _ = w.Write([]byte(`{"status":"success"}`))
		case "/api/tags", "/v1/api/tags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[]}`))
		default:
			t.Fatalf("unexpected endpoint path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("QUACKCESS_VECTOR_BACKEND", "http")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", server.URL+"/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "3")
	t.Setenv("QUACKCESS_VECTOR_TIMEOUT_SECONDS", "5")

	output, err := captureStdout(func() error {
		return run([]string{"install"})
	})
	if err != nil {
		t.Fatalf("run install: %v", err)
	}
	if !pullCalled {
		t.Fatal("expected model download call")
	}
	if pulledModel != defaultVectorModel {
		t.Fatalf("pulled model = %q, want %q", pulledModel, defaultVectorModel)
	}
	if !strings.Contains(output, "vector model ready: backend=http") {
		t.Fatalf("expected install readiness message, got %q", output)
	}
}

func TestRunInstallReportsMissingModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"data":[{"id":"other-model"},{"id":"backup"}]}`)
		case "/api/pull", "/v1/api/pull", "/api/tags", "/v1/api/tags", "/models", "/api/v1/models":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Fatalf("unexpected endpoint path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("QUACKCESS_VECTOR_BACKEND", "llama-cpp")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", server.URL+"/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3-embedding-0.6b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "3")

	err := run([]string{"install"})
	if err == nil {
		t.Fatalf("expected missing-model error")
	}
	if !strings.Contains(err.Error(), "vector model not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}
