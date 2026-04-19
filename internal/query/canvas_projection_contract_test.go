package query

import "testing"

func TestCanvasProjectionCanGenerateSQLForJoinedDocument(t *testing.T) {
	preview, err := PreviewCanvasDocument(CanvasDocument{
		ID:   "canvas-projection",
		Name: "joined-view",
		Spec: CanvasSpec{
			Nodes: []CanvasNode{
				{
					ID:     "customers",
					Kind:   CanvasNodeKindTable,
					Table:  "customers",
					Alias:  "c",
					Fields: []CanvasField{{Name: "id"}, {Name: "name"}},
				},
				{
					ID:     "orders",
					Kind:   CanvasNodeKindTable,
					Table:  "orders",
					Alias:  "o",
					Fields: []CanvasField{{Name: "id"}, {Name: "customer_id"}, {Name: "sku"}},
				},
			},
			Edges: []CanvasEdge{
				{
					ID:         "join-1",
					FromNode:   "customers",
					ToNode:     "orders",
					FromColumn: "id",
					ToColumn:   "customer_id",
					JoinType:   "LEFT JOIN",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if preview.DocumentID != "canvas-projection" {
		t.Fatalf("document id = %q, want canvas-projection", preview.DocumentID)
	}
	if preview.SQLText == "" {
		t.Fatal("expected SQL text")
	}
	if preview.SQLText != `SELECT "c"."id", "c"."name", "o"."id", "o"."customer_id", "o"."sku" FROM "customers" "c" LEFT JOIN "orders" "o" ON "c"."id" = "o"."customer_id"` {
		t.Fatalf("sql preview = %q, want joined SQL", preview.SQLText)
	}
}
