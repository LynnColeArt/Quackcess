package vector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	defaultEmbeddingBatchSize = 64
)

type RegisteredEmbeddingProvider struct {
	Name     string
	Model    string
	Provider EmbeddingProvider
}

type EmbeddingProviderRegistry struct {
	providers map[string]EmbeddingProvider
}

func NewEmbeddingProviderRegistry(entries ...RegisteredEmbeddingProvider) *EmbeddingProviderRegistry {
	registry := &EmbeddingProviderRegistry{
		providers: map[string]EmbeddingProvider{},
	}
	for _, entry := range entries {
		_ = registry.Register(entry.Name, entry.Model, entry.Provider)
	}
	return registry
}

func (r *EmbeddingProviderRegistry) Register(name, model string, provider EmbeddingProvider) error {
	if r == nil {
		return fmt.Errorf("embedding provider registry is not initialized")
	}
	name = strings.TrimSpace(strings.ToLower(name))
	model = strings.TrimSpace(strings.ToLower(model))
	if name == "" {
		return fmt.Errorf("provider name is required")
	}
	if model == "" {
		return fmt.Errorf("provider model is required")
	}
	if provider == nil {
		return fmt.Errorf("embedding provider is required")
	}
	if provider.Dimension() <= 0 {
		return fmt.Errorf("provider dimension must be positive")
	}
	if strings.TrimSpace(strings.ToLower(provider.Name())) != name {
		return fmt.Errorf("provider name %q does not match registry name %q", provider.Name(), name)
	}

	key := providerRegistryKey(name, model)
	if _, exists := r.providers[key]; exists {
		return fmt.Errorf("provider already registered: %s:%s", name, model)
	}
	r.providers[key] = provider
	return nil
}

func (r *EmbeddingProviderRegistry) Resolve(name, model string) (EmbeddingProvider, error) {
	if r == nil {
		return nil, fmt.Errorf("embedding provider registry is not configured")
	}
	name = strings.TrimSpace(strings.ToLower(name))
	model = strings.TrimSpace(strings.ToLower(model))
	if name == "" {
		return nil, fmt.Errorf("provider name is required")
	}
	if model == "" {
		return nil, fmt.Errorf("provider model is required")
	}

	provider, ok := r.providers[providerRegistryKey(name, model)]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s (%s)", name, model)
	}
	return provider, nil
}

func providerRegistryKey(name, model string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "::" + strings.ToLower(strings.TrimSpace(model))
}

type VectorBuildResult struct {
	Field       VectorField
	BatchSize   int
	Built       bool
	SkipReason  string
	VectorsByID map[string][]float64
}

type VectorBuildProgress struct {
	FieldID      string
	BatchIndex   int
	TotalBatches int
	BatchSize    int
	Processed    int
	Total        int
	Done         bool
}

type VectorBuildProgressHandler func(progress VectorBuildProgress)

type VectorBuildService struct {
	registry *EmbeddingProviderRegistry
	nowFn    func() time.Time
}

func NewVectorBuildService(registry *EmbeddingProviderRegistry) *VectorBuildService {
	return &VectorBuildService{
		registry: registry,
		nowFn:    time.Now,
	}
}

func (s *VectorBuildService) WithNow(fn func() time.Time) {
	if s == nil {
		return
	}
	if fn == nil {
		s.nowFn = time.Now
		return
	}
	s.nowFn = fn
}

func (s *VectorBuildService) BuildFromSourceTexts(
	ctx context.Context,
	field VectorField,
	sourceTexts map[string]string,
	sourceUpdatedAt time.Time,
	force bool,
	batchSize int,
	progressCallbacks ...VectorBuildProgressHandler,
) (VectorBuildResult, error) {
	if s == nil {
		return VectorBuildResult{}, fmt.Errorf("vector build service is not configured")
	}
	if s.registry == nil {
		return VectorBuildResult{}, fmt.Errorf("embedding provider registry is not configured")
	}
	field, err := CanonicalizeVectorField(field)
	if err != nil {
		return VectorBuildResult{}, err
	}

	if batchSize <= 0 {
		batchSize = defaultEmbeddingBatchSize
	}
	var progress VectorBuildProgressHandler
	if len(progressCallbacks) > 0 {
		progress = progressCallbacks[0]
	}
	now := s.now()
	if !s.ShouldBuild(field, sourceUpdatedAt, now, force) {
		return VectorBuildResult{
			Field:      field,
			BatchSize:  batchSize,
			Built:      false,
			SkipReason: "not stale",
		}, nil
	}

	provider, err := s.registry.Resolve(field.Provider, field.Model)
	if err != nil {
		return VectorBuildResult{}, err
	}
	if provider.Dimension() != field.Dimension {
		return VectorBuildResult{}, fmt.Errorf("provider dimension %d does not match vector field dimension %d", provider.Dimension(), field.Dimension)
	}

	ids := make([]string, 0, len(sourceTexts))
	for id := range sourceTexts {
		if strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	totalBatches := 0
	if len(ids) > 0 {
		totalBatches = (len(ids) + batchSize - 1) / batchSize
	}

	results := map[string][]float64{}
	processed := 0
	for start := 0; start < len(ids); start += batchSize {
		if err := ctx.Err(); err != nil {
			return VectorBuildResult{}, err
		}
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunkIDs := ids[start:end]
		chunkTexts := make([]string, len(chunkIDs))
		for i, id := range chunkIDs {
			chunkTexts[i] = sourceTexts[id]
		}
		vectors, err := provider.Embeddings(ctx, chunkTexts)
		if err != nil {
			return VectorBuildResult{}, err
		}
		if len(vectors) != len(chunkTexts) {
			return VectorBuildResult{}, fmt.Errorf("provider returned %d vectors for %d inputs", len(vectors), len(chunkTexts))
		}
		for i, value := range vectors {
			if len(value) != field.Dimension {
				return VectorBuildResult{}, fmt.Errorf(
					"provider returned vector dimension %d for record %s, expected %d",
					len(value),
					chunkIDs[i],
					field.Dimension,
				)
			}
			results[chunkIDs[i]] = append([]float64(nil), value...)
		}
		processed += len(chunkIDs)
		if progress != nil {
			progress(VectorBuildProgress{
				FieldID:      field.ID,
				BatchIndex:   (start / batchSize) + 1,
				TotalBatches: totalBatches,
				BatchSize:    len(chunkIDs),
				Processed:    processed,
				Total:        len(ids),
				Done:         false,
			})
		}
	}
	if progress != nil {
		progress(VectorBuildProgress{
			FieldID:      field.ID,
			BatchIndex:   totalBatches,
			TotalBatches: totalBatches,
			BatchSize:    0,
			Processed:    processed,
			Total:        len(ids),
			Done:         true,
		})
	}

	field.LastIndexedAt = now
	if !sourceUpdatedAt.IsZero() {
		field.SourceLastUpdatedAt = sourceUpdatedAt
	}
	return VectorBuildResult{
		Field:       field,
		BatchSize:   batchSize,
		Built:       true,
		VectorsByID: results,
	}, nil
}

func (s *VectorBuildService) ShouldBuild(field VectorField, sourceUpdatedAt time.Time, now time.Time, force bool) bool {
	if s == nil {
		return false
	}
	if force {
		return true
	}
	if field.StaleAfterHours < 0 {
		return false
	}
	return field.IsStale(sourceUpdatedAt, now)
}

func (s *VectorBuildService) ResolveProvider(field VectorField) (EmbeddingProvider, error) {
	if s == nil {
		return nil, fmt.Errorf("vector build service is not configured")
	}
	if s.registry == nil {
		return nil, fmt.Errorf("embedding provider registry is not configured")
	}
	field, err := CanonicalizeVectorField(field)
	if err != nil {
		return nil, err
	}
	return s.registry.Resolve(field.Provider, field.Model)
}

func (s *VectorBuildService) now() time.Time {
	if s == nil || s.nowFn == nil {
		return time.Now()
	}
	return s.nowFn()
}
