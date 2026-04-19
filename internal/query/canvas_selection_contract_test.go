package query

import "testing"

func TestCanvasSelectionNormalizationFiltersInvalidReferences(t *testing.T) {
	document := CanvasDocument{
		ID:   "canvas-selection-filter",
		Name: "selection",
		Spec: CanvasSpec{
			Nodes: []CanvasNode{
				{ID: "a", Kind: CanvasNodeKindTable, Table: "customers", Fields: []CanvasField{{Name: "id"}, {Name: "name"}}},
				{ID: "b", Kind: CanvasNodeKindTable, Table: "orders", Fields: []CanvasField{{Name: "id"}, {Name: "customer_id"}}},
			},
			Edges: []CanvasEdge{
				{
					ID:         "join-1",
					FromNode:   "a",
					ToNode:     "b",
					FromColumn: "id",
					ToColumn:   "customer_id",
					JoinType:   string(JoinLeft),
				},
			},
		},
		Selection: CanvasSelection{
			ActiveNodeID:    "a",
			ActiveEdgeID:    "missing-edge",
			SelectedNodeIDs: []string{"a", "a", "", "ghost"},
			SelectedEdgeIDs: []string{"join-1", "join-1", "other"},
		},
	}

	normalized, err := NormalizeCanvasDocument(document)
	if err != nil {
		t.Fatalf("normalize document: %v", err)
	}
	if normalized.Selection.ActiveNodeID != "a" {
		t.Fatalf("active node = %q, want a", normalized.Selection.ActiveNodeID)
	}
	if normalized.Selection.ActiveEdgeID != "" {
		t.Fatalf("active edge = %q, want empty", normalized.Selection.ActiveEdgeID)
	}
	if len(normalized.Selection.SelectedNodeIDs) != 1 || normalized.Selection.SelectedNodeIDs[0] != "a" {
		t.Fatalf("selected node ids = %#v, want [a]", normalized.Selection.SelectedNodeIDs)
	}
	if len(normalized.Selection.SelectedEdgeIDs) != 1 || normalized.Selection.SelectedEdgeIDs[0] != "join-1" {
		t.Fatalf("selected edge ids = %#v, want [join-1]", normalized.Selection.SelectedEdgeIDs)
	}
}
