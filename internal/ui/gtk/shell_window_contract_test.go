//go:build !gtk3
// +build !gtk3

package gtk

import (
	"errors"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
	"github.com/LynnColeArt/Quackcess/internal/ui/shell"
)

type shellWindowFakeRunner struct {
	calls  int
	result terminal.TerminalResult
	err    error
}

func (r *shellWindowFakeRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	r.calls++
	if r.err != nil {
		return terminal.TerminalResult{}, r.err
	}
	r.result.Input = input
	return r.result, nil
}

func TestShellWindowForwardsTerminalSubmitAndReturnsProjection(t *testing.T) {
	runner := &shellWindowFakeRunner{result: terminal.TerminalResult{
		Kind:      terminal.TerminalKindQuery,
		SQLText:   "SELECT * FROM products",
		Columns:   []string{"id", "name"},
		Rows:      [][]any{{int64(1), "A"}, {int64(2), "B"}},
		RowCount:  2,
		ErrorText: "",
	}}
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))
	window, err := NewShellWindow(bridge, nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}

	if err := window.SubmitTerminalCommand("SELECT * FROM products"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if window.Projection().LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want query", window.Projection().LastKind)
	}
	if window.Projection().RowCount != 2 {
		t.Fatalf("row count = %d, want 2", window.Projection().RowCount)
	}
	if window.Projection().SQLText != "SELECT * FROM products" {
		t.Fatalf("sql = %q, want %q", window.Projection().SQLText, "SELECT * FROM products")
	}
	if window.Projection().OutputText == "" {
		t.Fatal("expected output text in projection")
	}
	if window.Projection().ResultColumns[0] != "id" || window.Projection().ResultColumns[1] != "name" {
		t.Fatalf("result columns = %#v", window.Projection().ResultColumns)
	}
	if window.Projection().ResultRows[0] != "1\tA" {
		t.Fatalf("row 0 = %q", window.Projection().ResultRows[0])
	}
}

func TestShellWindowForwardsF12ToProjection(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	window := newTestWindowFromState(state, t)

	if err := window.HandleKey("F12"); err != nil {
		t.Fatalf("handle key: %v", err)
	}
	if !window.Projection().ConsoleVisible {
		t.Fatal("expected console visible")
	}
}

func TestShellWindowReportsUnavailableAtRun(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	window := newTestWindowFromState(state, t)
	if err := window.Run(); err != ErrGTKUnavailable {
		t.Fatalf("run err = %v, want %v", err, ErrGTKUnavailable)
	}
}

func TestShellWindowRejectsUnknownShortcut(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	window := newTestWindowFromState(state, t)

	if err := window.HandleKey("F11"); err == nil {
		t.Fatal("expected unknown shortcut error")
	}
}

func TestShellWindowPropagatesTerminalFailure(t *testing.T) {
	fail := errors.New("boom")
	state := appstate.NewShellState(nil)
	runner := &shellWindowFakeRunner{err: fail}
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))
	window, err := NewShellWindow(bridge, nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}

	err = window.SubmitTerminalCommand("bad")
	if err != fail {
		t.Fatalf("expected %v, got %v", fail, err)
	}
}

func TestShellWindowForwardsAddCanvasNodeAndProjectsDirtyState(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &shellWindowFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	window := newTestWindowFromState(state, t)
	if err := window.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := window.AddCanvasNode(query.CanvasNode{
		ID:     "shipments",
		Table:  "shipments",
		Fields: []query.CanvasField{{Name: "id"}},
	}); err != nil {
		t.Fatalf("add canvas node: %v", err)
	}
	if !window.Projection().CanvasDirty {
		t.Fatal("expected dirty canvas after adding node")
	}
}

func TestShellWindowForwardsCanvasEdgeLifecycleAndProjectsDirtyState(t *testing.T) {
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &shellWindowFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"region"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"},{"name":"total"}]}
			],"edges":[]}`,
		},
	})
	window := newTestWindowFromState(state, t)
	if err := window.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := window.AddCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(query.JoinInner),
	}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if !window.Projection().CanvasDirty {
		t.Fatal("expected dirty after add edge")
	}

	if err := window.SetCanvasNodeFields("customers", []string{"id"}); err != nil {
		t.Fatalf("set node fields: %v", err)
	}

	if err := window.PatchCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "region",
		ToColumn:   "total",
		JoinType:   string(query.JoinLeft),
	}); err != nil {
		t.Fatalf("patch edge: %v", err)
	}
	if !window.Projection().CanvasDirty {
		t.Fatal("expected dirty after patch edge")
	}

	if err := window.DeleteCanvasEdge("join-1"); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	if !window.Projection().CanvasDirty {
		t.Fatal("expected dirty after delete edge")
	}
}

func TestShellWindowForwardsRunCanvasActionToRunner(t *testing.T) {
	runner := &shellWindowFakeRunner{
		result: terminal.TerminalResult{
			Kind:     terminal.TerminalKindQuery,
			RowCount: 1,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &shellWindowFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	window, err := NewShellWindow(NewShellBridge(shell.NewShellPresenter(model, nil)), nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}

	if err := window.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := window.RunActiveCanvas(); err != nil {
		t.Fatalf("run canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(window.Projection().CanvasSQLPreview, "\"customers\"") {
		t.Fatalf("canvas sql preview = %q", window.Projection().CanvasSQLPreview)
	}
	if window.Projection().LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want %q", window.Projection().LastKind, terminal.TerminalKindQuery)
	}
}

func TestShellWindowForwardsCanvasCreateAndProjectsRunnerCall(t *testing.T) {
	runner := &shellWindowFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas project-overview",
		},
	}
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	window, err := NewShellWindow(NewShellBridge(shell.NewShellPresenter(model, nil)), nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}

	if err := window.CreateCanvas("project-overview"); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, `\canvas new project-overview`) {
		t.Fatalf("runner input = %q, want \\canvas new project-overview", runner.result.Input)
	}
}

func TestShellWindowForwardsCanvasRenameAndUpdatesActiveCanvas(t *testing.T) {
	runner := &shellWindowFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed canvas sales -> sales-archive",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &shellWindowFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	window, err := NewShellWindow(NewShellBridge(shell.NewShellPresenter(model, nil)), nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}

	if err := window.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := window.RenameCanvas("sales", "sales-archive"); err != nil {
		t.Fatalf("rename canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if window.Projection().ActiveCanvas != "sales-archive" {
		t.Fatalf("active canvas = %q, want sales-archive", window.Projection().ActiveCanvas)
	}
	if !strings.Contains(runner.result.Input, `\canvas rename sales sales-archive`) {
		t.Fatalf("runner input = %q, want \\canvas rename sales sales-archive", runner.result.Input)
	}
}

func TestShellWindowForwardsCanvasDelete(t *testing.T) {
	runner := &shellWindowFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted canvas sales",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &shellWindowFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	window, err := NewShellWindow(NewShellBridge(shell.NewShellPresenter(model, nil)), nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}

	if err := window.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := window.DeleteCanvas("sales"); err != nil {
		t.Fatalf("delete canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if window.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", window.Projection().ActiveCanvas)
	}
	if !strings.Contains(runner.result.Input, `\canvas delete sales`) {
		t.Fatalf("runner input = %q, want \\canvas delete sales", runner.result.Input)
	}
}

type shellWindowFakeCatalogExplorer struct {
	canvases map[string]string
}

func (f *shellWindowFakeCatalogExplorer) ListTables() ([]string, error) {
	return nil, nil
}

func (f *shellWindowFakeCatalogExplorer) ListViews() ([]string, error) {
	return nil, nil
}

func (f *shellWindowFakeCatalogExplorer) ListCanvases() ([]string, error) {
	names := make([]string, 0, len(f.canvases))
	for n := range f.canvases {
		names = append(names, n)
	}
	return names, nil
}

func (f *shellWindowFakeCatalogExplorer) LoadCanvasSpec(name string) (string, error) {
	if spec, ok := f.canvases[name]; ok {
		return spec, nil
	}
	return "", errors.New("canvas not found")
}

func newTestWindowFromState(state *appstate.ShellState, t *testing.T) *ShellWindow {
	t.Helper()
	model := shell.NewShellModel(appstate.NewShellCommandBus(&shellWindowFakeRunner{}, state))
	window, err := NewShellWindow(NewShellBridge(shell.NewShellPresenter(model, nil)), nil)
	if err != nil {
		t.Fatalf("new shell window: %v", err)
	}
	return window
}
