//go:build !gtk3
// +build !gtk3

package gtk

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
	"github.com/LynnColeArt/Quackcess/internal/ui/shell"
)

const (
	shellResponsivenessBudget = 500 * time.Millisecond
)

func TestShellWindowLargeCanvasInteractionsMeetResponsivenessBudget(t *testing.T) {
	runner := &shellWindowFakeRunner{
		result: terminal.TerminalResult{
			Kind:     terminal.TerminalKindQuery,
			RowCount: 1,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &shellWindowFakeCatalogExplorer{
		canvases: map[string]string{
			"big-canvas": buildLargeCanvasSpec(250),
		},
	})
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	window := newWindowForPerformanceBudget(t, model)

	start := time.Now()
	if err := window.SetActiveCanvas("big-canvas"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if elapsed := time.Since(start); elapsed > shellResponsivenessBudget {
		t.Fatalf("active canvas load took %s, exceeds budget %s", elapsed, shellResponsivenessBudget)
	}

	start = time.Now()
	if err := window.RunActiveCanvas(); err != nil {
		t.Fatalf("run active canvas: %v", err)
	}
	if elapsed := time.Since(start); elapsed > shellResponsivenessBudget {
		t.Fatalf("canvas run took %s, exceeds budget %s", elapsed, shellResponsivenessBudget)
	}

	start = time.Now()
	if err := window.HandleKey("F12"); err != nil {
		t.Fatalf("toggle console: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("console toggle took %s, exceeds budget %s", elapsed, 50*time.Millisecond)
	}

	if window.Projection().CanvasDraftSpec == "" {
		t.Fatal("expected canvas draft spec in projection")
	}
	if !strings.Contains(window.Projection().CanvasSQLPreview, "\"customers_0001\"") {
		t.Fatalf("canvas SQL preview missing expected node: %q", window.Projection().CanvasSQLPreview)
	}
}

func newWindowForPerformanceBudget(t *testing.T, model *shell.ShellModel) *ShellWindow {
	t.Helper()
	window, err := NewShellWindow(NewShellBridge(shell.NewShellPresenter(model, nil)), nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}
	return window
}

func buildLargeCanvasSpec(nodeCount int) string {
	type spec struct {
		Version string           `json:"version"`
		Nodes   []map[string]any `json:"nodes"`
		Edges   []map[string]any `json:"edges"`
		Title   string           `json:"title"`
	}

	payload := spec{
		Version: "1.0.0",
		Title:   "performance",
		Nodes:   make([]map[string]any, 0, nodeCount),
		Edges:   make([]map[string]any, 0, max(0, nodeCount-1)),
	}

	for i := 0; i < nodeCount; i++ {
		payload.Nodes = append(payload.Nodes, map[string]any{
			"id":              fmt.Sprintf("n%04d", i),
			"kind":            "table",
			"table":           fmt.Sprintf("customers_%04d", i),
			"x":               float64(i % 20),
			"y":               float64(i / 20),
			"fields":          []map[string]any{{"name": "id"}},
			"width":           220,
			"height":          180,
			"alias":           fmt.Sprintf("customers_%04d", i),
			"selected_fields": []string{"id"},
		})
		if i == 0 {
			continue
		}
		payload.Edges = append(payload.Edges, map[string]any{
			"id":         fmt.Sprintf("e%04d", i),
			"kind":       "join",
			"from":       fmt.Sprintf("n%04d", i-1),
			"to":         fmt.Sprintf("n%04d", i),
			"fromColumn": "id",
			"toColumn":   "id",
			"joinType":   "LEFT JOIN",
		})
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func max(lhs, rhs int) int {
	if lhs < rhs {
		return rhs
	}
	return lhs
}
