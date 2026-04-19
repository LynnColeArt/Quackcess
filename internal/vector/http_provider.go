package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPEmbeddingProviderConfig struct {
	Name       string
	Model      string
	Endpoint   string
	Dimension  int
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type HTTPEmbeddingProvider struct {
	name      string
	model     string
	endpoint  *url.URL
	dimension int
	apiKey    string
	client    *http.Client
}

func NewHTTPEmbeddingProvider(cfg HTTPEmbeddingProviderConfig) (*HTTPEmbeddingProvider, error) {
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)

	if cfg.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("provider model is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("provider endpoint is required")
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("provider dimension must be positive")
	}

	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", cfg.Endpoint, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("provider endpoint must include scheme and host: %q", cfg.Endpoint)
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}

	return &HTTPEmbeddingProvider{
		name:      cfg.Name,
		model:     cfg.Model,
		endpoint:  parsed,
		dimension: cfg.Dimension,
		apiKey:    cfg.APIKey,
		client:    client,
	}, nil
}

func (p *HTTPEmbeddingProvider) Name() string {
	return p.name
}

func (p *HTTPEmbeddingProvider) Dimension() int {
	return p.dimension
}

func (p *HTTPEmbeddingProvider) Embeddings(ctx context.Context, inputs []string) ([][]float64, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is not initialized")
	}
	if p.client == nil {
		p.client = &http.Client{Timeout: 30 * time.Second}
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("embedding input is required")
	}

	normalized := make([]string, len(inputs))
	for i, text := range inputs {
		normalized[i] = strings.TrimSpace(text)
	}

	payload, err := json.Marshal(map[string]any{
		"model": p.model,
		"input": normalized,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("provider request failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read provider response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("provider request failed with status %d: %s", response.StatusCode, trimText(string(body), 600))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("provider response body is empty")
	}

	return parseEmbeddingResponse(body, len(normalized), p.dimension)
}

type openAIEmbeddingData struct {
	Index     int         `json:"index"`
	Embedding []float64   `json:"embedding"`
	Object    string      `json:"object"`
	Error     interface{} `json:"error"`
}

type openAIEmbeddingResponse struct {
	Data      []openAIEmbeddingData `json:"data"`
	Object    string                `json:"object"`
	Usage     map[string]any        `json:"usage"`
	Embedding []float64             `json:"embedding"`
}

func parseEmbeddingResponse(raw []byte, expectedCount int, expectedDimension int) ([][]float64, error) {
	var response openAIEmbeddingResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode provider response: %w", err)
	}

	if len(response.Data) > 0 {
		if len(response.Data) != expectedCount {
			return nil, fmt.Errorf("provider returned %d vectors, expected %d", len(response.Data), expectedCount)
		}
		vectors := make([][]float64, expectedCount)
		used := make(map[int]struct{}, expectedCount)
		for _, item := range response.Data {
			if len(item.Embedding) != expectedDimension {
				return nil, fmt.Errorf("provider returned embedding dimension %d, expected %d", len(item.Embedding), expectedDimension)
			}
			index := item.Index
			if index < 0 || index >= expectedCount {
				return nil, fmt.Errorf("provider returned invalid index %d", index)
			}
			if _, duplicate := used[index]; duplicate {
				return nil, fmt.Errorf("provider returned duplicate index %d", index)
			}
			used[index] = struct{}{}
			values := make([]float64, len(item.Embedding))
			copy(values, item.Embedding)
			vectors[index] = values
		}
		for i := 0; i < expectedCount; i++ {
			if _, ok := used[i]; !ok {
				return nil, fmt.Errorf("provider omitted index %d", i)
			}
		}
		return vectors, nil
	}

	if len(response.Embedding) > 0 {
		if expectedCount != 1 {
			return nil, fmt.Errorf("provider returned a single embedding for %d inputs", expectedCount)
		}
		if len(response.Embedding) != expectedDimension {
			return nil, fmt.Errorf("provider returned embedding dimension %d, expected %d", len(response.Embedding), expectedDimension)
		}
		return [][]float64{response.Embedding}, nil
	}

	return nil, fmt.Errorf("provider response is missing embeddings")
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
