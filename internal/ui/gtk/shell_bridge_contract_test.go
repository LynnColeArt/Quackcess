package gtk

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
	"github.com/LynnColeArt/Quackcess/internal/ui/shell"
)

type bridgeFakeRunner struct {
	calls  int
	result terminal.TerminalResult
	err    error
}

func (b *bridgeFakeRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	b.calls++
	if b.err != nil {
		return terminal.TerminalResult{}, b.err
	}
	b.result.Input = input
	return b.result, nil
}

type bridgeFakeCatalogExplorer struct {
	canvases map[string]string
}

func (f *bridgeFakeCatalogExplorer) ListTables() ([]string, error) {
	return nil, nil
}

func (f *bridgeFakeCatalogExplorer) ListViews() ([]string, error) {
	return nil, nil
}

func (f *bridgeFakeCatalogExplorer) ListCanvases() ([]string, error) {
	names := make([]string, 0, len(f.canvases))
	for name := range f.canvases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (f *bridgeFakeCatalogExplorer) LoadCanvasSpec(name string) (string, error) {
	if spec, ok := f.canvases[name]; ok {
		return spec, nil
	}
	return "", fmt.Errorf("canvas not found: %s", name)
}

func TestGtkBridgeForwardsTerminalSubmitToPresenter(t *testing.T) {
	runner := &bridgeFakeRunner{result: terminal.TerminalResult{Kind: terminal.TerminalKindQuery}}
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	presenter := shell.NewShellPresenter(shell.NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)
	bridge := NewShellBridge(presenter)

	if err := bridge.SubmitTerminalInput("SELECT 1"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
}

func TestGtkBridgeForwardsF12ToProjection(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := shell.NewShellModel(appstate.NewShellCommandBus(nil, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := bridge.HandleKey("F12"); err != nil {
		t.Fatalf("handle key: %v", err)
	}
	if !bridge.Projection().ConsoleVisible {
		t.Fatalf("expected console visible")
	}
}

func TestGtkBridgeReturnsErrorWhenNoPresenter(t *testing.T) {
	bridge := NewShellBridge(nil)
	if err := bridge.HandleKey("F12"); err == nil {
		t.Fatal("expected handle key without presenter to fail")
	}
	if err := bridge.SubmitTerminalInput("SELECT 1"); err == nil {
		t.Fatal("expected submit without presenter to fail")
	}
}

func TestGtkBridgeRejectsUnhandledShortcut(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	bridge := NewShellBridge(shell.NewShellPresenter(shell.NewShellModel(appstate.NewShellCommandBus(nil, state)), nil))

	err := bridge.HandleKey("F11")
	if err == nil {
		t.Fatal("expected unhandled shortcut to fail")
	}
}

func TestGtkBridgeForwardsAddCanvasNodeToPresenter(t *testing.T) {
	explorer := &bridgeFakeCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := shell.NewShellModel(appstate.NewShellCommandBus(nil, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bridge.AddCanvasNode(query.CanvasNode{
		ID:     "shipments",
		Table:  "shipments",
		Fields: []query.CanvasField{{Name: "id"}},
	}); err != nil {
		t.Fatalf("add canvas node: %v", err)
	}
	if !bridge.Projection().CanvasDirty {
		t.Fatal("expected projection dirty after add")
	}
}

func TestGtkBridgeForwardsCanvasEdgeLifecycleToPresenter(t *testing.T) {
	explorer := &bridgeFakeCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"region"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"},{"name":"total"}]}
			],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := shell.NewShellModel(appstate.NewShellCommandBus(nil, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}

	if err := bridge.AddCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
		JoinType:   string(query.JoinInner),
	}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if !bridge.Projection().CanvasDirty {
		t.Fatal("expected projection dirty after add")
	}

	if err := bridge.PatchCanvasEdge(query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "region",
		ToColumn:   "total",
		JoinType:   string(query.JoinLeft),
	}); err != nil {
		t.Fatalf("patch edge: %v", err)
	}
	if !bridge.Projection().CanvasDirty {
		t.Fatal("expected projection dirty after patch")
	}
	if err := bridge.DeleteCanvasEdge("join-1"); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	spec, err := query.ParseCanvasSpec([]byte(bridge.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(spec.Edges))
	}
}

func TestGtkBridgeForwardsRunCanvasToTerminalRunner(t *testing.T) {
	explorer := &bridgeFakeCatalogExplorer{
		canvases: map[string]string{
			"analytics": `{"nodes":[{"id":"orders","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &bridgeFakeRunner{result: terminal.TerminalResult{Kind: terminal.TerminalKindQuery, RowCount: 1}}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := model.SetActiveCanvas("analytics"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bridge.RunActiveCanvas(); err != nil {
		t.Fatalf("run canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, "\"orders\"") {
		t.Fatalf("runner input = %q, expected SQL", runner.result.Input)
	}
}

func TestGtkBridgePropagatesTerminalFailure(t *testing.T) {
	fail := errors.New("fail")
	runner := &bridgeFakeRunner{err: fail}
	state := appstate.NewShellState(nil)
	bridge := NewShellBridge(shell.NewShellPresenter(shell.NewShellModel(appstate.NewShellCommandBus(runner, state)), nil))

	err := bridge.SubmitTerminalInput("bad")
	if err != fail {
		t.Fatalf("err = %v, want %v", err, fail)
	}
}

func TestGtkBridgeForwardsCanvasCreateToPresenter(t *testing.T) {
	runner := &bridgeFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas project-overview",
		},
	}
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := bridge.CreateCanvas("project-overview"); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, `\canvas new project-overview`) {
		t.Fatalf("runner input = %q, want \\canvas new project-overview", runner.result.Input)
	}
}

func TestGtkBridgeForwardsCanvasRenameToPresenterAndProjectsActiveCanvas(t *testing.T) {
	runner := &bridgeFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed canvas sales -> sales-archive",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &bridgeFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bridge.RenameCanvas("sales", "sales-archive"); err != nil {
		t.Fatalf("rename canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if bridge.Projection().ActiveCanvas != "sales-archive" {
		t.Fatalf("active canvas = %q, want sales-archive", bridge.Projection().ActiveCanvas)
	}
	if !strings.Contains(runner.result.Input, `\canvas rename sales sales-archive`) {
		t.Fatalf("runner input = %q, want \\canvas rename sales sales-archive", runner.result.Input)
	}
}

func TestGtkBridgeForwardsCanvasDeleteToPresenter(t *testing.T) {
	runner := &bridgeFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted sales",
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &bridgeFakeCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	})
	model := shell.NewShellModel(appstate.NewShellCommandBus(runner, state))
	bridge := NewShellBridge(shell.NewShellPresenter(model, nil))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bridge.DeleteCanvas("sales"); err != nil {
		t.Fatalf("delete canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if bridge.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", bridge.Projection().ActiveCanvas)
	}
	if !strings.Contains(runner.result.Input, `\canvas delete sales`) {
		t.Fatalf("runner input = %q, want \\canvas delete sales", runner.result.Input)
	}
}
