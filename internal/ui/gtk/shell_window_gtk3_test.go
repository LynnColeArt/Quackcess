//go:build gtk3
// +build gtk3

package gtk

import (
	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"testing"
)

func TestNextCanvasDraftEdgeIDFindsFreeID(t *testing.T) {
	spec := query.CanvasSpec{
		Edges: []query.CanvasEdge{
			{ID: "edge-1"},
			{ID: "edge-3"},
		},
	}
	got := nextCanvasDraftEdgeID(spec)
	if got != "edge-2" {
		t.Fatalf("next edge id = %q, want edge-2", got)
	}
}

func TestDistancePointToSegmentCalculatesNearestDistance(t *testing.T) {
	got := distancePointToSegment(3, 4, 0, 0, 6, 0)
	if got < 0 || got > 4 {
		t.Fatalf("distance = %v, want 4", got)
	}
}

func TestCanvasNodeHitTestHonorsBounds(t *testing.T) {
	spec := query.CanvasSpec{
		Nodes: []query.CanvasNode{
			{ID: "n1", X: 10, Y: 5, Width: 50, Height: 20},
			{ID: "n2", X: 70, Y: 5, Width: 40, Height: 20},
		},
	}
	if got := hitCanvasNodeAt(spec, 20, 10); got != "n1" {
		t.Fatalf("hit = %q, want n1", got)
	}
	if got := hitCanvasNodeAt(spec, 120, 10); got != "" {
		t.Fatalf("hit = %q, want empty", got)
	}
}

func TestRefreshCanvasEditPanelsClearsMissingNodeSelection(t *testing.T) {
	spec := query.CanvasSpec{
		Nodes: []query.CanvasNode{
			{
				ID:     "n1",
				Kind:   query.CanvasNodeKindTable,
				Table:  "customers",
				Alias:  "customers",
				Fields: []query.CanvasField{{Name: "id"}},
				X:      0,
				Y:      0,
				Width:  100,
				Height: 50,
			},
		},
	}
	raw, err := query.MarshalCanvasSpec(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	window := &ShellWindow{
		projection: appstate.ShellProjection{
			CanvasDraftSpec: string(raw),
		},
		activeCanvasNodeID: "missing-node",
	}

	window.refreshCanvasEditPanels()

	if got := window.activeCanvasNodeID; got != "" {
		t.Fatalf("activeCanvasNodeID = %q, want empty", got)
	}
}
