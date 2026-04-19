package appstate

import (
	"strings"
	"testing"
)

func TestCanvasSQLPreviewReflectsActiveCanvasDraftWithDefaultLimit(t *testing.T) {
	canvases := &fakeCatalogExplorer{
		canvases: []string{"orders"},
		specs: map[string]string{
			"orders": `{"nodes":[{"id":"o","kind":"table","table":"orders","alias":"o","fields":[{"name":"id"},{"name":"status"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, canvases)
	if err := state.SetActiveCanvas("orders"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}

	projection := state.Projection()
	if projection.CanvasSQLPreview != `SELECT "o"."id", "o"."status" FROM "orders" "o" LIMIT 200` {
		t.Fatalf("canvas SQL preview = %q", projection.CanvasSQLPreview)
	}
}

func TestCanvasSQLPreviewRebuildsAfterNodeFieldMutationAndClearsOnSetActiveCanvas(t *testing.T) {
	state := NewShellStateWithCatalogExplorer(nil, &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"orders","kind":"table","table":"orders","alias":"o","fields":[{"name":"id"},{"name":"status"}]}],"edges":[]}`,
		},
	})
	if err := state.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if before := state.Projection().CanvasSQLPreview; !strings.Contains(before, `"id"`) {
		t.Fatalf("initial preview = %q, expected full field projection", before)
	}
	if err := NewShellCommandBus(nil, state).Dispatch(Action{
		Kind:    ActionSetNodeFields,
		Payload: `{"node_id":"orders","fields":["status"]}`,
	}); err != nil {
		t.Fatalf("set node fields: %v", err)
	}
	preview := state.Projection().CanvasSQLPreview
	if !strings.Contains(preview, `"status"`) {
		t.Fatalf("preview = %q, expected selected field", preview)
	}
	if strings.Contains(preview, `"id"`) {
		t.Fatalf("preview = %q, expected field selection to narrow columns", preview)
	}

	state.ClearActiveCanvas()
	if preview := state.Projection().CanvasSQLPreview; preview != "" {
		t.Fatalf("canvas SQL preview after clear = %q, want empty", preview)
	}
}

func TestCanvasSQLPreviewDefaultsToEmptyWhenActiveCanvasSpecIsMalformed(t *testing.T) {
	state := NewShellStateWithCatalogExplorer(nil, &fakeCatalogExplorer{
		canvases: []string{"broken"},
		specs: map[string]string{
			"broken": `not-json`,
		},
	})
	if err := state.SetActiveCanvas("broken"); err == nil {
		t.Fatal("expected set active canvas to fail for malformed JSON")
	}
	if projection := state.Projection(); projection.CanvasSQLPreview != "" {
		t.Fatalf("canvas SQL preview = %q, want empty", projection.CanvasSQLPreview)
	}
}
