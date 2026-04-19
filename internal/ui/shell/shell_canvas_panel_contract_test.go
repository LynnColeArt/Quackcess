package shell

import (
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestShellModelProjectsCanvasPanelStateForActiveCanvas(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}

	projection := model.Projection()
	if projection.ActiveCanvas != "sales" {
		t.Fatalf("active canvas = %q, want sales", projection.ActiveCanvas)
	}
	if projection.CanvasStatus != "canvas loaded" {
		t.Fatalf("canvas status = %q, want canvas loaded", projection.CanvasStatus)
	}
	if !strings.Contains(projection.CanvasDraftSpec, "\"id\"") {
		t.Fatalf("canvas draft spec = %q, want includes id field", projection.CanvasDraftSpec)
	}
}

func TestShellModelProjectionShowsCanvasListAndDraftState(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"inventory": `{"nodes":[{"id":"i","kind":"table","table":"products","fields":[{"name":"sku"}]}],"edges":[]}`,
			"sales":     `{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.SetCanvasDraft(`{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set canvas draft: %v", err)
	}
	projection := model.Projection()
	if len(projection.CatalogCanvases) != 2 {
		t.Fatalf("canvas count = %d, want 2", len(projection.CatalogCanvases))
	}
	if !projection.CanvasDirty {
		t.Fatal("expected dirty state after draft edit")
	}
}

func TestShellModelRevertCanvasDraftInProjection(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.SetCanvasDraft(`{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set canvas draft: %v", err)
	}
	if err := model.RevertCanvas(); err != nil {
		t.Fatalf("revert canvas: %v", err)
	}
	projection := model.Projection()
	if projection.CanvasDirty {
		t.Fatal("expected dirty false after revert")
	}
	if !strings.Contains(projection.CanvasDraftSpec, `"id"`) {
		t.Fatalf("canvas draft spec = %q, want original", projection.CanvasDraftSpec)
	}
}

func TestShellModelProjectsCanvasSQLPreview(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id","alias":"order_id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	projection := model.Projection()
	if projection.CanvasSQLPreview == "" {
		t.Fatal("expected canvas SQL preview")
	}
	if !strings.Contains(projection.CanvasSQLPreview, "SELECT") {
		t.Fatalf("canvas SQL preview = %q, want select statement", projection.CanvasSQLPreview)
	}
	if !strings.Contains(projection.CanvasSQLPreview, `orders`) {
		t.Fatalf("canvas SQL preview = %q, want orders table", projection.CanvasSQLPreview)
	}
	if !strings.Contains(projection.CanvasSQLPreview, "LIMIT 200") {
		t.Fatalf("canvas SQL preview = %q, expected LIMIT 200", projection.CanvasSQLPreview)
	}
}

func TestShellModelCanvasMoveUpdatesProjectionThroughModelAPI(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}]},{"id":"o","kind":"table","table":"orders","fields":[{"name":"customer_id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.MoveCanvasNode("c", 0, 12); err != nil {
		t.Fatalf("move canvas node: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(model.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if spec.Nodes[0].Y != 12 {
		t.Fatalf("node y = %v, want %v", spec.Nodes[0].Y, 12)
	}
}

func TestShellModelCanvasSQLPreviewRefreshesAfterEdgeMutation(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"id"},{"name":"customer_id"}]}
			],"edges":[]}`,
		},
	})
	model := NewShellModel(appstate.NewShellCommandBus(nil, state))
	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := model.AddCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(query.JoinLeft),
	}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	projection := model.Projection()
	if projection.CanvasSQLPreview == "" {
		t.Fatal("expected SQL preview after edge add")
	}
	if !strings.Contains(projection.CanvasSQLPreview, "LEFT JOIN") {
		t.Fatalf("preview = %q, want LEFT JOIN", projection.CanvasSQLPreview)
	}
	if !strings.Contains(projection.CanvasSQLPreview, "\"customers\"") || !strings.Contains(projection.CanvasSQLPreview, "\"orders\"") {
		t.Fatalf("preview = %q, want includes customers and orders", projection.CanvasSQLPreview)
	}
	if err := model.DeleteCanvasEdge("join-1"); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	preview := model.Projection().CanvasSQLPreview
	if strings.Contains(preview, "LEFT JOIN") {
		t.Fatal("expected preview to remove join after edge delete")
	}
}
