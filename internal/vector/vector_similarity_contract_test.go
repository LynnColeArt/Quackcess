package vector

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type fakeProvider struct {
	dimension int
	vectors   [][]float64
}

func (p *fakeProvider) Name() string {
	return "fake"
}

func (p *fakeProvider) Dimension() int {
	return p.dimension
}

func (p *fakeProvider) Embeddings(_ context.Context, inputs []string) ([][]float64, error) {
	_ = inputs
	if len(p.vectors) != 1 {
		return nil, nil
	}
	return p.vectors, nil
}

func TestSearchByVectorReturnsRankedResultsAndTrim(t *testing.T) {
	query := []float64{1, 0}
	candidates := map[string][]float64{
		"a": {0.8, 0.6},
		"b": {1, 0},
		"c": {0, 1},
		"d": {0.2, 0.8},
	}
	got, err := SearchByVector(query, candidates, 3)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("matches = %d, want 3", len(got))
	}
	if got[0].ID != "b" || got[0].Score <= got[1].Score {
		t.Fatalf("expected b to be top match: %#v", got)
	}
}

func TestSearchByVectorRejectsDimensionMismatch(t *testing.T) {
	query := []float64{1, 2, 3}
	candidates := map[string][]float64{
		"ok": {1, 0, 0},
		"bad": {1, 0},
	}
	if _, err := SearchByVector(query, candidates, 10); err == nil {
		t.Fatalf("expected mismatch error")
	} else if !strings.Contains(err.Error(), "vector dimension mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchByTextUsesProviderAndLimits(t *testing.T) {
	provider := &fakeProvider{
		dimension: 2,
		vectors:   [][]float64{{1, 0}},
	}
	candidates := map[string][]float64{
		"first":  {1, 0},
		"second": {0.5, 0.5},
		"third":  {0, 1},
	}

	got, err := SearchByText(context.Background(), provider, "find similar", candidates, 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0], SimilarityMatch{ID: "first", Score: 1}) {
		t.Fatalf("top match = %#v, want first score 1", got[0])
	}
}
