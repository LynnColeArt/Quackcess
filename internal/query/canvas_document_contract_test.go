package query

import "testing"

func TestNormalizeCanvasDocumentCarriesUIMetadataAndSelectionState(t *testing.T) {
	raw := []byte(`{
		"id":"doc-1",
		"name":" Sales Overview ",
		"kind":"query",
		"title":" Customer Summary ",
		"description":"  ",
		"tags":["Sales","sales","analytics",""],
		"selection":{
			"activeNodeID":"customers",
			"activeEdgeID":"missing-edge",
			"selectedNodeIDs":["customers","ghost","customers"],
			"selectedEdgeIDs":["join-1","join-1"]
		},
		"spec": {
			"nodes":[
				{
					"id":"customers",
					"kind":"table",
					"table":"customers",
					"alias":"c",
					"x":-10,
					"y":5,
					"width":0,
					"height":60,
					"fields":[{"name":"id"},{"name":"name"}],
					"selected_fields":["id","id","name","missing"]
				}
			],
			"edges":[
				{
					"id":"join-1",
					"kind":"join",
					"from":"customers",
					"to":"customers",
					"joinType":"LEFT",
					"fromColumn":"id",
					"toColumn":"id",
					"label":"identity"
				}
			]
		}
		}`)

	document, err := ParseCanvasDocument(raw)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}
	if document.ID != "doc-1" {
		t.Fatalf("id = %q, want doc-1", document.ID)
	}
	if len(document.Tags) != 2 || document.Tags[0] != "sales" || document.Tags[1] != "analytics" {
		t.Fatalf("tags = %#v, want two normalized tags", document.Tags)
	}
	if document.Selection.ActiveEdgeID != "" {
		t.Fatalf("active edge id should be cleared to empty, got %q", document.Selection.ActiveEdgeID)
	}
	if len(document.Selection.SelectedNodeIDs) != 1 || document.Selection.SelectedNodeIDs[0] != "customers" {
		t.Fatalf("selected node ids = %#v, want [customers]", document.Selection.SelectedNodeIDs)
	}
	if len(document.Selection.SelectedEdgeIDs) != 1 || document.Selection.SelectedEdgeIDs[0] != "join-1" {
		t.Fatalf("selected edge ids = %#v, want [join-1]", document.Selection.SelectedEdgeIDs)
	}

	if len(document.Spec.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(document.Spec.Nodes))
	}
	node := document.Spec.Nodes[0]
	if node.X != 0 {
		t.Fatalf("x = %v, want 0", node.X)
	}
	if node.Y != 5 {
		t.Fatalf("y = %v, want 5", node.Y)
	}
	if node.Width != 240 {
		t.Fatalf("width = %v, want 240", node.Width)
	}
	if node.Height != 80 {
		t.Fatalf("height = %v, want 80", node.Height)
	}
	if len(node.SelectedFields) != 2 || node.SelectedFields[0] != "id" || node.SelectedFields[1] != "name" {
		t.Fatalf("selected fields = %#v, want [id name]", node.SelectedFields)
	}
}

func TestNormalizeCanvasSpecRejectsUnknownJoinColumns(t *testing.T) {
	_, err := ParseCanvasSpec([]byte(`{
		"nodes":[
			{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]},
			{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"}]}
		],
		"edges":[
			{"id":"join-1","kind":"join","from":"customers","to":"orders","fromColumn":"id","toColumn":"missing"}
		]
	}`))
	if err == nil {
		t.Fatal("expected missing to-column error")
	}
}

func TestCanvasPreviewFromDocumentGeneratesSQL(t *testing.T) {
	preview, err := PreviewCanvasDocument(CanvasDocument{
		ID: "canvas-preview",
		Spec: CanvasSpec{
			Nodes: []CanvasNode{
				{
					ID:     "products",
					Kind:   CanvasNodeKindTable,
					Table:  "products",
					Alias:  "p",
					Fields: []CanvasField{{Name: "sku"}, {Name: "name"}},
				},
			},
			Edges: []CanvasEdge{},
		},
	})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.DocumentID != "canvas-preview" {
		t.Fatalf("document id = %q, want canvas-preview", preview.DocumentID)
	}
	if preview.SQLText != `SELECT "p"."sku", "p"."name" FROM "products" "p"` {
		t.Fatalf("sql preview = %q, want %q", preview.SQLText, `SELECT "p"."sku", "p"."name" FROM "products" "p"`)
	}
}
