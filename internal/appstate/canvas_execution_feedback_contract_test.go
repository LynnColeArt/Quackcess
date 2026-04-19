package appstate

import (
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestMapCanvasErrorToNodeIDPrefersAliasAndTableMatches(t *testing.T) {
	spec := `{"nodes":[{"id":"customers","kind":"table","table":"customers","alias":"c","fields":[{"name":"id"}]},{"id":"ghost","kind":"table","table":"ghost","fields":[{"name":"id"}]}],"edges":[{"id":"join-1","kind":"join","from":"customers","to":"ghost","fromColumn":"id","toColumn":"id","joinType":"LEFT"}]}`
	nodeID, ok := mapCanvasErrorToNodeID(spec, `Catalog Error: Table "ghost" does not exist`)
	if !ok {
		t.Fatal("expected error mapping to succeed")
	}
	if nodeID != "ghost" {
		t.Fatalf("nodeID = %q, want %q", nodeID, "ghost")
	}
}

func TestMapCanvasErrorToNodeIDHandlesQualifiedColumns(t *testing.T) {
	spec := `{"nodes":[{"id":"customers","kind":"table","table":"customers","alias":"c","fields":[{"name":"id"}]}],"edges":[]}`
	nodeID, ok := mapCanvasErrorToNodeID(spec, `catalog error: reference to invalid column "customers"."status" in FROM clause`)
	if !ok {
		t.Fatal("expected qualified error mapping to succeed")
	}
	if nodeID != "customers" {
		t.Fatalf("nodeID = %q, want %q", nodeID, "customers")
	}
}

func TestCanvasRunFailureMapsToNodeStatusInProjection(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:      terminal.TerminalKindError,
			ErrorText: `Catalog Error: Table "ghost" does not exist`,
			SQLText:   `SELECT "g"."id" FROM "ghost" "g"`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"ghost","kind":"table","table":"ghost","alias":"g","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionRunCanvas}); err != nil {
		t.Fatalf("run canvas: %v", err)
	}
	projection := state.Projection()
	if !strings.Contains(projection.CanvasStatus, "execution failed at node ghost") {
		t.Fatalf("canvas status = %q", projection.CanvasStatus)
	}
}
