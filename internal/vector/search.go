package vector

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type EmbeddingProvider interface {
	Name() string
	Dimension() int
	Embeddings(ctx context.Context, inputs []string) ([][]float64, error)
}

type SimilarityMatch struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

func SearchByText(
	ctx context.Context,
	provider EmbeddingProvider,
	queryText string,
	candidates map[string][]float64,
	limit int,
) ([]SimilarityMatch, error) {
	if provider == nil {
		return nil, fmt.Errorf("embedding provider is required")
	}
	if provider.Dimension() <= 0 {
		return nil, fmt.Errorf("provider embedding dimension must be positive")
	}
	if len(queryText) == 0 {
		return nil, fmt.Errorf("query text is required")
	}

	vectors, err := provider.Embeddings(ctx, []string{queryText})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embedding provider returned %d vectors, expected 1", len(vectors))
	}
	if len(vectors[0]) != provider.Dimension() {
		return nil, fmt.Errorf("provider returned vector dimension %d, expected %d", len(vectors[0]), provider.Dimension())
	}
	return SearchByVector(vectors[0], candidates, limit)
}

func SearchByVector(query []float64, candidates map[string][]float64, limit int) ([]SimilarityMatch, error) {
	if len(query) == 0 {
		return nil, fmt.Errorf("query vector is required")
	}
	if limit < 0 {
		return nil, fmt.Errorf("limit must be >= 0")
	}

	matches := make([]SimilarityMatch, 0, len(candidates))
	for candidateID, vector := range candidates {
		if len(vector) != len(query) {
			return nil, fmt.Errorf("vector dimension mismatch for candidate %s", candidateID)
		}
		matches = append(matches, SimilarityMatch{
			ID:    candidateID,
			Score: cosineSimilarity(query, vector),
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Score > matches[j].Score
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func ValidateCandidateDimension(expectedDimension int, candidates map[string][]float64) error {
	if expectedDimension <= 0 {
		return fmt.Errorf("query vector dimension must be positive")
	}
	for candidateID, vector := range candidates {
		if len(vector) != expectedDimension {
			return fmt.Errorf("candidate %s has dimension %d, expected %d", candidateID, len(vector), expectedDimension)
		}
	}
	return nil
}

func cosineSimilarity(a, b []float64) float64 {
	dot := 0.0
	magA := 0.0
	magB := 0.0
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}
