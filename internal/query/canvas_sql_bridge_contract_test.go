package query

import (
	"testing"
)

func TestGenerateSQLFromCanvasSingleTable(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "products",
				Kind:  CanvasNodeKindTable,
				Table: "products",
				Alias: "p",
				Fields: []CanvasField{
					{Name: "sku"},
					{Name: "name"},
				},
			},
		},
	}

	sqlText, err := GenerateSQLFromCanvas(spec)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got, want := sqlText.SQL, `SELECT "p"."sku", "p"."name" FROM "products" "p"`; got != want {
		t.Fatalf("sql = %q, want %q", got, want)
	}
}

func TestGenerateSQLFromCanvasFillsDefaultAlias(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "events",
				Kind:  CanvasNodeKindTable,
				Table: "events",
			},
		},
	}

	sqlText, err := GenerateSQLFromCanvas(spec)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if got, want := sqlText.SQL, `SELECT * FROM "events" "events"`; got != want {
		t.Fatalf("sql = %q, want %q", got, want)
	}
}

func TestGenerateSQLFromCanvasJoinSpec(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Alias: "c",
				Fields: []CanvasField{
					{Name: "id"},
					{Name: "name"},
				},
			},
			{
				ID:    "orders",
				Kind:  CanvasNodeKindTable,
				Table: "orders",
				Alias: "o",
				Fields: []CanvasField{
					{Name: "total", Alias: "order_total"},
					{Name: "customer_id"},
				},
			},
		},
		Edges: []CanvasEdge{
			{
				ID:         "join-1",
				Kind:       CanvasEdgeKindJoin,
				FromNode:   "customers",
				ToNode:     "orders",
				JoinType:   string(JoinLeft),
				FromColumn: "id",
				ToColumn:   "customer_id",
			},
		},
	}

	sqlText, err := GenerateSQLFromCanvas(spec)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	want := `SELECT "c"."id", "c"."name", "o"."total" AS "order_total", "o"."customer_id" FROM "customers" "c" LEFT JOIN "orders" "o" ON "c"."id" = "o"."customer_id"`
	if sqlText.SQL != want {
		t.Fatalf("sql = %q, want %q", sqlText.SQL, want)
	}
}

func TestGenerateSQLFromCanvasWithLimitAppliesLimitByDefault(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:     "orders",
				Kind:   CanvasNodeKindTable,
				Table:  "orders",
				Alias:  "o",
				Fields: []CanvasField{{Name: "id"}},
			},
		},
	}
	sqlText, err := GenerateSQLFromCanvasWithLimit(spec, 25)
	if err != nil {
		t.Fatalf("generate with limit: %v", err)
	}
	if sqlText.SQL != `SELECT "o"."id" FROM "orders" "o" LIMIT 25` {
		t.Fatalf("sql = %q, want %q", sqlText.SQL, `SELECT "o"."id" FROM "orders" "o" LIMIT 25`)
	}
}

func TestGenerateSQLFromCanvasWithLimitSkipsNonPositiveLimit(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "orders",
				Kind:  CanvasNodeKindTable,
				Table: "orders",
				Alias: "o",
				Fields: []CanvasField{
					{Name: "id"},
				},
			},
		},
	}
	sqlText, err := GenerateSQLFromCanvasWithLimit(spec, 0)
	if err != nil {
		t.Fatalf("generate with limit: %v", err)
	}
	if sqlText.SQL != `SELECT "o"."id" FROM "orders" "o"` {
		t.Fatalf("sql = %q, want %q", sqlText.SQL, `SELECT "o"."id" FROM "orders" "o"`)
	}
}

func TestQueryGraphFromCanvasRejectsUnknownEdgeNode(t *testing.T) {
	_, err := QueryGraphFromCanvas(CanvasSpec{
		Nodes: []CanvasNode{
			{ID: "orders", Kind: CanvasNodeKindTable, Table: "orders"},
		},
		Edges: []CanvasEdge{
			{FromNode: "orders", ToNode: "missing", JoinType: string(JoinInner), FromColumn: "id", ToColumn: "order_id"},
		},
	})
	if err == nil {
		t.Fatal("expected edge node error")
	}
}

func TestGenerateSQLFromCanvasUsesSelectedFields(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{
				ID:    "customers",
				Kind:  CanvasNodeKindTable,
				Table: "customers",
				Alias: "c",
				Fields: []CanvasField{
					{Name: "id"},
					{Name: "name"},
				},
				SelectedFields: []string{"name"},
			},
		},
	}

	sqlText, err := GenerateSQLFromCanvas(spec)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if sqlText.SQL != `SELECT "c"."name" FROM "customers" "c"` {
		t.Fatalf("sql = %q, want %q", sqlText.SQL, `SELECT "c"."name" FROM "customers" "c"`)
	}
}
