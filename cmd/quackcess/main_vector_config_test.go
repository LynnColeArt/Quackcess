package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/vector"
)

func TestVectorProviderConfigFromEnvUsesDefaultsWhenNotSet(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", "")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "")

	cfg, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !enabled {
		t.Fatal("expected vector provider to be enabled")
	}
	if cfg.backend != "cpu" {
		t.Fatalf("backend = %q, want cpu", cfg.backend)
	}
	if cfg.http.Name != defaultVectorCPUProviderName {
		t.Fatalf("name = %q, want %q", cfg.http.Name, defaultVectorCPUProviderName)
	}
	if cfg.http.Model != defaultVectorCPUModel {
		t.Fatalf("model = %q, want %q", cfg.http.Model, defaultVectorCPUModel)
	}
	if cfg.http.Endpoint != "" {
		t.Fatalf("endpoint = %q, want empty for default cpu backend", cfg.http.Endpoint)
	}
	if cfg.http.Dimension != defaultVectorDimension {
		t.Fatalf("dimension = %d, want %d", cfg.http.Dimension, defaultVectorDimension)
	}
	if cfg.http.Timeout != defaultVectorTimeoutSeconds*time.Second {
		t.Fatalf("timeout = %v, want %v", cfg.http.Timeout, defaultVectorTimeoutSeconds*time.Second)
	}
}

func TestVectorProviderConfigFromEnvParsesRequiredFields(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "http")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", "http://localhost:11434/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_PROVIDER", "")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3.5-0.8b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "1024")
	t.Setenv("QUACKCESS_VECTOR_API_KEY", "abc")
	t.Setenv("QUACKCESS_VECTOR_TIMEOUT_SECONDS", "7")

	cfg, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !enabled {
		t.Fatal("expected vector provider to be enabled")
	}
	if cfg.http.Name != "qwen-local" {
		t.Fatalf("name = %q, want qwen-local", cfg.http.Name)
	}
	if cfg.http.Dimension != 1024 {
		t.Fatalf("dimension = %d, want 1024", cfg.http.Dimension)
	}
	if cfg.http.Timeout != 7*time.Second {
		t.Fatalf("timeout = %v, want 7s", cfg.http.Timeout)
	}
	if _, err := vector.NewHTTPEmbeddingProvider(cfg.http); err != nil {
		t.Fatalf("provider: %v", err)
	}
}

func TestVectorProviderConfigFromEnvDefaultsModelWhenMissing(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "http")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", "http://localhost:11434/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_PROVIDER", "qwen-local")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "1024")

	cfg, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !enabled {
		t.Fatal("expected vector provider to be enabled")
	}
	if cfg.http.Model != defaultVectorModel {
		t.Fatalf("model = %q, want %q", cfg.http.Model, defaultVectorModel)
	}

	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3.5-0.8b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "bad")
	if _, _, err := vectorProviderConfigFromEnv(); err == nil {
		t.Fatal("expected invalid dimension error")
	}
}

func TestVectorProviderConfigFromEnvRequiresValidTimeoutWhenSet(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "http")
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", "http://localhost:11434/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3.5-0.8b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "1024")
	t.Setenv("QUACKCESS_VECTOR_TIMEOUT_SECONDS", "0")

	if _, _, err := vectorProviderConfigFromEnv(); err == nil {
		t.Fatal("expected timeout validation error")
	}

	t.Setenv("QUACKCESS_VECTOR_TIMEOUT_SECONDS", "abc")
	if _, _, err := vectorProviderConfigFromEnv(); err == nil {
		t.Fatal("expected timeout parse error")
	}
}

func TestVectorProviderConfigFromEnvRejectsInvalidBackend(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "nope")
	if _, _, err := vectorProviderConfigFromEnv(); err == nil {
		t.Fatal("expected backend parse error")
	}
}

func TestVectorProviderConfigFromEnvSupportsCPUBackend(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "cpu")
	t.Setenv("QUACKCESS_VECTOR_PROVIDER", "")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "768")
	t.Setenv("QUACKCESS_VECTOR_CPU_SEED", "123")

	cfg, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !enabled {
		t.Fatal("expected vector provider to be enabled")
	}
	if cfg.backend != "cpu" {
		t.Fatalf("backend = %q, want cpu", cfg.backend)
	}
	if cfg.http.Name != defaultVectorCPUProviderName {
		t.Fatalf("name = %q, want %q", cfg.http.Name, defaultVectorCPUProviderName)
	}
	if cfg.http.Model != defaultVectorCPUModel {
		t.Fatalf("model = %q, want %q", cfg.http.Model, defaultVectorCPUModel)
	}
	if cfg.http.Dimension != 768 {
		t.Fatalf("dimension = %d, want 768", cfg.http.Dimension)
	}
	if cfg.cpuSeed != 123 {
		t.Fatalf("seed = %d, want 123", cfg.cpuSeed)
	}
	if cfg.http.Endpoint != "" {
		t.Fatalf("endpoint = %q, want empty for CPU backend", cfg.http.Endpoint)
	}
}

func TestVectorProviderConfigFromEnvSupportsLlamaCppBackendAlias(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_BACKEND", "llama-cpp")
	t.Setenv("QUACKCESS_VECTOR_PROVIDER", "")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "")

	cfg, enabled, err := vectorProviderConfigFromEnv()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if !enabled {
		t.Fatal("expected vector provider to be enabled")
	}
	if cfg.backend != "llamacpp" {
		t.Fatalf("backend = %q, want llamacpp", cfg.backend)
	}
	if cfg.http.Name != defaultVectorLlamaProviderName {
		t.Fatalf("name = %q, want %q", cfg.http.Name, defaultVectorLlamaProviderName)
	}
	if cfg.http.Model != defaultVectorLlamaModel {
		t.Fatalf("model = %q, want %q", cfg.http.Model, defaultVectorLlamaModel)
	}
	if cfg.http.Endpoint != defaultVectorLlamaEndpoint {
		t.Fatalf("endpoint = %q, want %q", cfg.http.Endpoint, defaultVectorLlamaEndpoint)
	}
}

func TestNewVectorServiceReturnsProviderSetupErrorForInvalidConfig(t *testing.T) {
	t.Setenv("QUACKCESS_VECTOR_ENDPOINT", "http://localhost:11434/v1/embeddings")
	t.Setenv("QUACKCESS_VECTOR_MODEL", "qwen3.5-0.8b")
	t.Setenv("QUACKCESS_VECTOR_DIMENSION", "bad")

	database, err := db.Bootstrap(filepath.Join(t.TempDir(), "vector.db"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := newVectorService(database)
	if _, err := service.RebuildVector("vf-missing", false); err == nil {
		t.Fatal("expected provider setup error")
	} else if !strings.Contains(err.Error(), "vector provider setup") {
		t.Fatalf("err = %q, expected vector provider setup", err.Error())
	}
}
