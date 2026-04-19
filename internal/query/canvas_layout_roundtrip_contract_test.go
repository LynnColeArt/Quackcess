package query

import (
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCanvasLayoutCanRoundtripThroughCatalog(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "catalog.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := catalog.NewCanvasRepository(database.SQL)

	spec := CanvasSpec{
		Name: "sales-overview",
		Nodes: []CanvasNode{
			{ID: "customers", Kind: CanvasNodeKindTable, Table: "customers", Alias: "c"},
			{ID: "orders", Kind: CanvasNodeKindTable, Table: "orders", Alias: "o"},
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

	payload, err := MarshalCanvasSpec(spec)
	if err != nil {
		t.Fatalf("marshal canvas spec: %v", err)
	}

	if err := repo.Create(catalog.Canvas{
		ID:       "canvas-1",
		Name:     "sales overview",
		Kind:     "query",
		SpecJSON: string(payload),
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	canvases, err := repo.List()
	if err != nil {
		t.Fatalf("list canvases: %v", err)
	}
	if len(canvases) != 1 {
		t.Fatalf("len(canvases) = %d, want 1", len(canvases))
	}

	loaded, err := ParseCanvasSpec([]byte(canvases[0].SpecJSON))
	if err != nil {
		t.Fatalf("parse loaded spec: %v", err)
	}
	if len(loaded.Nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(loaded.Nodes))
	}
	if loaded.Edges[0].ToNode != "orders" {
		t.Fatalf("edge target = %q, want orders", loaded.Edges[0].ToNode)
	}
}
