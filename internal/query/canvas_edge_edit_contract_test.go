package query

import "testing"

func TestAddCanvasEdgeAddsJoinAfterValidation(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Alias: "c",
				Fields: []CanvasField{{Name: "id"}, {Name: "name"}},
			},
			{
				ID:    "orders",
				Kind:  CanvasNodeKindTable,
				Table: "orders",
				Alias: "o",
				Fields: []CanvasField{{Name: "id"}, {Name: "customer_id"}},
			},
		},
		Edges: []CanvasEdge{},
	}

	edge := CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(JoinLeft),
	}
	updated, err := AddCanvasEdge(spec, edge)
	if err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if len(updated.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(updated.Edges))
	}
	if updated.Edges[0].JoinType != string(JoinLeft) {
		t.Fatalf("join type = %q, want %q", updated.Edges[0].JoinType, string(JoinLeft))
	}
}

func TestPatchCanvasEdgeUpdatesJoinColumns(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "a",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Alias: "c",
				Fields: []CanvasField{
					{Name: "id"},
					{Name: "name"},
				},
			},
			{
				ID:    "b",
				Kind:  CanvasNodeKindTable,
				Table: "orders",
				Alias: "o",
				Fields: []CanvasField{
					{Name: "customer_id"},
					{Name: "id"},
				},
			},
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
	}

	updated, err := PatchCanvasEdge(spec, CanvasEdge{
		ID:         "join-1",
		FromNode:   "a",
		ToNode:     "b",
		FromColumn: "name",
		ToColumn:   "id",
		JoinType:   string(JoinInner),
	})
	if err != nil {
		t.Fatalf("patch edge: %v", err)
	}
	if len(updated.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(updated.Edges))
	}
	if updated.Edges[0].FromColumn != "name" || updated.Edges[0].ToColumn != "id" {
		t.Fatalf("edge columns = %q -> %q, want name -> id", updated.Edges[0].FromColumn, updated.Edges[0].ToColumn)
	}
}

func TestDeleteCanvasEdgeRemovesJoinFromSpec(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Fields: []CanvasField{{Name: "id"}},
			},
			{
				ID:    "orders",
				Kind:  CanvasNodeKindTable,
				Table: "orders",
				Fields: []CanvasField{{Name: "customer_id"}},
			},
		},
		Edges: []CanvasEdge{
			{
				ID:         "join-1",
				FromNode:   "customers",
				ToNode:     "orders",
				FromColumn: "id",
				ToColumn:   "customer_id",
				JoinType:   string(JoinLeft),
			},
		},
	}

	updated, err := DeleteCanvasEdge(spec, "join-1")
	if err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	if len(updated.Edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(updated.Edges))
	}
}

func TestPatchCanvasEdgeRejectsUnknownEdgeID(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Fields: []CanvasField{{Name: "id"}},
			},
			{
				ID:    "orders",
				Kind:  CanvasNodeKindTable,
				Table: "orders",
				Fields: []CanvasField{{Name: "customer_id"}},
			},
		},
		Edges: []CanvasEdge{
			{
				ID:         "join-1",
				FromNode:   "customers",
				ToNode:     "orders",
				FromColumn: "id",
				ToColumn:   "customer_id",
				JoinType:   string(JoinLeft),
			},
		},
	}
	_, err := PatchCanvasEdge(spec, CanvasEdge{
		ID:         "missing",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(JoinInner),
	})
	if err == nil {
		t.Fatal("expected unknown edge error")
	}
}
