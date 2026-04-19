package vector

import (
	"context"
	"strings"
	"testing"
)

func TestSearchByTextValidatesProviderDimension(t *testing.T) {
	provider := &fakeProvider{
		dimension: 3,
		vectors:   [][]float64{{1, 0, 0}},
	}
	candidates := map[string][]float64{
		"same":   {1, 0, 0},
		"other":  {0, 1, 0},
	}
	if _, err := SearchByText(context.Background(), provider, "query", candidates, 5); err != nil {
		t.Fatalf("search: %v", err)
	}
}

func TestSearchByTextRejectsProviderDimensionMismatch(t *testing.T) {
	provider := &fakeProvider{
		dimension: 4,
		vectors:   [][]float64{{1, 0, 0, 0}},
	}
	candidates := map[string][]float64{
		"same": {1, 0, 0},
	}
	if _, err := SearchByText(context.Background(), provider, "query", candidates, 5); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "vector dimension mismatch for candidate same") {
		t.Fatalf("unexpected error: %v", err)
	}
}
