package vector

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

type CPUEmbeddingProviderConfig struct {
	Name      string
	Model     string
	Dimension int
	Seed      uint64
}

type CPUEmbeddingProvider struct {
	name      string
	model     string
	dimension int
	seed      uint64
}

func NewCPUEmbeddingProvider(cfg CPUEmbeddingProviderConfig) (*CPUEmbeddingProvider, error) {
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Model = strings.TrimSpace(cfg.Model)

	if cfg.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("provider model is required")
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("provider dimension must be positive")
	}

	return &CPUEmbeddingProvider{
		name:      cfg.Name,
		model:     cfg.Model,
		dimension: cfg.Dimension,
		seed:      cfg.Seed,
	}, nil
}

func (p *CPUEmbeddingProvider) Name() string {
	return p.name
}

func (p *CPUEmbeddingProvider) Dimension() int {
	return p.dimension
}

func (p *CPUEmbeddingProvider) Embeddings(ctx context.Context, inputs []string) ([][]float64, error) {
	if p == nil {
		return nil, fmt.Errorf("provider is not initialized")
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("embedding input is required")
	}

	embeddings := make([][]float64, len(inputs))
	for i, raw := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		normalized := strings.TrimSpace(raw)
		embeddings[i] = p.embed(normalized)
	}
	return embeddings, nil
}

func (p *CPUEmbeddingProvider) embed(input string) []float64 {
	seed := p.seed
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(p.model))
	_, _ = hash.Write([]byte("|"))
	_, _ = hash.Write([]byte(input))
	_, _ = hash.Write([]byte("|"))
	_, _ = hash.Write([]byte(p.name))
	state := hash.Sum64()
	if state == 0 {
		state = 0x9e3779b97f4a7c15
	}
	if seed != 0 {
		state ^= seed
	}

	values := make([]float64, p.dimension)
	magnitude := 0.0
	for i := 0; i < p.dimension; i++ {
		state = splitmix64(state)
		values[i] = scaleToFloat64(state)
		magnitude += values[i] * values[i]
	}
	if magnitude == 0 {
		for i := range values {
			values[i] = 1.0 / float64(p.dimension)
		}
		return values
	}
	invMagnitude := 1.0 / math.Sqrt(magnitude)
	for i := range values {
		values[i] *= invMagnitude
	}
	return values
}

func splitmix64(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func scaleToFloat64(value uint64) float64 {
	return (float64(value>>11)/(float64(uint64(1)<<53)))*2 - 1
}
