package vector

import (
	"testing"
	"time"
)

func TestVectorIndexStaleWhenNeverIndexed(t *testing.T) {
	field := VectorField{
		ID:              "vf",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       16,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
	}
	now := time.Date(2025, 4, 1, 12, 0, 0, 0, time.UTC)
	if !field.IsStale(time.Time{}, now) {
		t.Fatal("expected stale when never indexed")
	}
}

func TestVectorIndexStaleWhenSourceUpdatedAfterIndex(t *testing.T) {
	now := time.Date(2025, 4, 1, 12, 0, 0, 0, time.UTC)
	field := VectorField{
		ID:              "vf",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       16,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 24,
		LastIndexedAt:   now.Add(-30 * time.Minute),
	}
	if !field.IsStale(now.Add(-1*time.Minute), now) {
		t.Fatal("expected stale because source updated after index time")
	}
}

func TestVectorIndexStaleRespectsConfiguredWindow(t *testing.T) {
	indexed := time.Date(2025, 4, 1, 8, 0, 0, 0, time.UTC)
	now := time.Date(2025, 4, 2, 7, 0, 0, 0, time.UTC)
	field := VectorField{
		ID:              "vf",
		TableName:       "docs",
		SourceColumn:    "content",
		Dimension:       16,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 12,
		LastIndexedAt:   indexed,
	}
	if !field.IsStale(time.Time{}, now) {
		t.Fatal("expected stale after window expiration")
	}
}
