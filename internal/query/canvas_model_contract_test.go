package query

import (
	"strings"
	"testing"
)

func TestCanvasSpecRejectsInvalidJSON(t *testing.T) {
	_, err := ParseCanvasSpec([]byte(`{"nodes":[`))
	if err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestCanvasSpecRequiresNodes(t *testing.T) {
	_, err := ParseCanvasSpec([]byte(`{"nodes":[]}`))
	if err == nil {
		t.Fatal("expected missing nodes error")
	}
}

func TestParseCanvasSpecDefaultsAliasAndJoinType(t *testing.T) {
	raw := []byte(`{
		"version":"0.1.0",
		"nodes":[
			{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}]},
			{"id":"orders","kind":"table","table":"orders","alias":"o","fields":[{"name":"total"},{"name":"customer_id"}]}
		],
		"edges":[{"id":"join-1","kind":"join","from":"customers","to":"orders","joinType":"LEFT JOIN","fromColumn":"id","toColumn":"customer_id"}]
	}`)

	spec, err := ParseCanvasSpec(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Version != "0.1.0" {
		t.Fatalf("version = %q, want 0.1.0", spec.Version)
	}
	if spec.Nodes[0].Alias != "customers" {
		t.Fatalf("alias = %q, want customers", spec.Nodes[0].Alias)
	}
	if spec.Edges[0].JoinType != "LEFT JOIN" {
		t.Fatalf("join type = %q, want LEFT JOIN", spec.Edges[0].JoinType)
	}
}

func TestCanvasSpecRejectsDuplicateNodeID(t *testing.T) {
	_, err := ParseCanvasSpec([]byte(`{
		"nodes":[
			{"id":"n1","kind":"table","table":"customers"},
			{"id":"n1","kind":"table","table":"orders"}
		]
	}`))
	if err == nil {
		t.Fatal("expected duplicate node id error")
	}
}

func TestCanvasSpecRejectsDuplicateAlias(t *testing.T) {
	_, err := ParseCanvasSpec([]byte(`{
		"nodes":[
			{"id":"n1","kind":"table","table":"customers","alias":"same"},
			{"id":"n2","kind":"table","table":"orders","alias":"same"}
		]
	}`))
	if err == nil {
		t.Fatal("expected duplicate alias error")
	}
}

func TestCanvasSpecRejectsUnknownEdgeEndpoint(t *testing.T) {
	_, err := ParseCanvasSpec([]byte(`{
		"nodes":[{"id":"n1","kind":"table","table":"customers"}],
		"edges":[{"id":"join-1","kind":"join","from":"n1","to":"missing","fromColumn":"id","toColumn":"customer_id"}]
	}`))
	if err == nil {
		t.Fatal("expected edge endpoint error")
	}
	if !strings.Contains(err.Error(), "missing node") {
		t.Fatalf("unexpected error %q", err)
	}
}

func TestMarshalCanvasSpecEnforcesNormalization(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{ID: "n1", Kind: "table", Table: "customers"},
		},
	}

	payload, err := MarshalCanvasSpec(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	loaded, err := ParseCanvasSpec(payload)
	if err != nil {
		t.Fatalf("parse marshaled: %v", err)
	}
	if loaded.Version != canvasSpecVersion {
		t.Fatalf("version = %q, want %q", loaded.Version, canvasSpecVersion)
	}
	if loaded.Nodes[0].Alias != "customers" {
		t.Fatalf("alias = %q, want customers", loaded.Nodes[0].Alias)
	}
}

func TestAddCanvasNodeAddsNodeWithDefaultsAndNormalization(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{ID: "customers", Kind: CanvasNodeKindTable, Table: "customers", Fields: []CanvasField{{Name: "id"}}},
		},
		Edges: []CanvasEdge{},
	}

	updated, err := AddCanvasNode(spec, CanvasNode{
		ID:     "orders",
		Table:  "orders",
		X:      -12,
		Y:      10,
		Width:  30,
		Height: 20,
		Fields: []CanvasField{{Name: "id"}, {Name: " total "}, {Name: ""}},
	})
	if err != nil {
		t.Fatalf("add canvas node: %v", err)
	}
	if len(updated.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(updated.Nodes))
	}
	node := updated.Nodes[1]
	if node.ID != "orders" {
		t.Fatalf("node id = %q, want orders", node.ID)
	}
	if node.Alias != "orders" {
		t.Fatalf("node alias = %q, want orders", node.Alias)
	}
	if node.X != 0 {
		t.Fatalf("node x = %v, want 0", node.X)
	}
	if node.Width != canvasNodeMinSize || node.Height != canvasNodeMinSize {
		t.Fatalf("node dimensions = %v x %v, want %v x %v", node.Width, node.Height, canvasNodeMinSize, canvasNodeMinSize)
	}
	if len(node.Fields) != 2 {
		t.Fatalf("fields = %#v, want [id total]", node.Fields)
	}
}

func TestAddCanvasNodeRejectsDuplicateNodeID(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{ID: "n1", Kind: CanvasNodeKindTable, Table: "customers"},
		},
	}
	_, err := AddCanvasNode(spec, CanvasNode{
		ID:    "n1",
		Table: "orders",
	})
	if err == nil {
		t.Fatal("expected duplicate node id error")
	}
}

func TestAddCanvasNodeRejectsNodeMissingTable(t *testing.T) {
	spec := CanvasSpec{
		Nodes: []CanvasNode{
			{ID: "n1", Kind: CanvasNodeKindTable, Table: "customers"},
		},
	}
	_, err := AddCanvasNode(spec, CanvasNode{
		ID: "n2",
	})
	if err == nil {
		t.Fatal("expected table is required error")
	}
}
