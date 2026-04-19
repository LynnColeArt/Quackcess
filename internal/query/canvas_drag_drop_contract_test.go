package query

import "testing"

func TestMoveCanvasNodeUpdatesCoordinates(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Alias: "c",
				X:     10,
				Y:     10,
				Fields: []CanvasField{
					{Name: "id"},
				},
			},
		},
		Edges: []CanvasEdge{},
	}

	mutated, err := MoveCanvasNode(spec, "customers", -5, 3.5)
	if err != nil {
		t.Fatalf("move node: %v", err)
	}
	if mutated.Nodes[0].X != 0 {
		t.Fatalf("x = %v, want 0", mutated.Nodes[0].X)
	}
	if mutated.Nodes[0].Y != 3.5 {
		t.Fatalf("y = %v, want 3.5", mutated.Nodes[0].Y)
	}
}

func TestMoveCanvasNodeRequiresExistingNode(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Fields: []CanvasField{{Name: "id"}},
			},
		},
	}
	_, err := MoveCanvasNode(spec, "missing", 1, 2)
	if err == nil {
		t.Fatal("expected unknown node error")
	}
}

func TestSetCanvasNodeSelectedFieldsFiltersToKnownFields(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Fields: []CanvasField{
					{Name: "id"},
					{Name: "name"},
					{Name: "status"},
				},
			},
		},
		Edges: []CanvasEdge{},
	}

	updated, err := SetCanvasNodeSelectedFields(spec, "customers", []string{"name", "missing", "name", "id"})
	if err != nil {
		t.Fatalf("set selected fields: %v", err)
	}
	if len(updated.Nodes[0].SelectedFields) != 2 {
		t.Fatalf("selected fields = %#v, want [name id]", updated.Nodes[0].SelectedFields)
	}
	if updated.Nodes[0].SelectedFields[0] != "name" || updated.Nodes[0].SelectedFields[1] != "id" {
		t.Fatalf("selected fields = %#v, want [name id]", updated.Nodes[0].SelectedFields)
	}
}
