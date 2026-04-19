package appstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type fakeTerminalRunner struct {
	calls  int
	result terminal.TerminalResult
	err    error
}

type fakeCatalogExplorer struct {
	tables   []string
	views    []string
	canvases []string
	specs    map[string]string
}

func (f *fakeCatalogExplorer) ListTables() ([]string, error) {
	return f.tables, nil
}

func (f *fakeCatalogExplorer) ListViews() ([]string, error) {
	return f.views, nil
}

func (f *fakeCatalogExplorer) ListCanvases() ([]string, error) {
	return f.canvases, nil
}

func (f *fakeCatalogExplorer) LoadCanvasSpec(name string) (string, error) {
	if f == nil || f.specs == nil {
		return "", nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("canvas name is required")
	}
	if spec, ok := f.specs[name]; ok {
		return spec, nil
	}
	return "", fmt.Errorf("canvas not found: %s", name)
}

func (f *fakeTerminalRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	f.calls++
	if f.err != nil {
		return terminal.TerminalResult{}, f.err
	}
	f.result.Input = input
	return f.result, nil
}

func TestShellCommandBusTogglesConsoleVisibilityViaAction(t *testing.T) {
	console := terminal.NewEventConsole(10)
	state := NewShellState(console)
	bus := NewShellCommandBus(nil, state)

	if err := bus.Dispatch(Action{Kind: ActionToggleConsole}); err != nil {
		t.Fatalf("dispatch toggle: %v", err)
	}
	if !state.IsConsoleVisible() {
		t.Fatal("expected console visible after toggle")
	}

	if err := bus.Dispatch(Action{Kind: ActionToggleConsole}); err != nil {
		t.Fatalf("dispatch second toggle: %v", err)
	}
	if state.IsConsoleVisible() {
		t.Fatal("expected console hidden after second toggle")
	}
	if console.IsVisible() {
		t.Fatal("expected underlying console to be hidden after second toggle")
	}
}

func TestShellCommandBusRunsTerminalAndProjectsLastResult(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:      terminal.TerminalKindQuery,
			SQLText:   "SELECT * FROM products",
			Columns:   []string{"id", "name"},
			Rows:      [][]any{{int64(1), "A"}, {int64(2), "B"}},
			RowCount:  3,
			ErrorText: "",
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: "SELECT * FROM products",
	}); err != nil {
		t.Fatalf("dispatch terminal action: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}

	projection := state.Projection()
	if projection.LastInput != "SELECT * FROM products" {
		t.Fatalf("last input = %q, want SELECT * FROM products", projection.LastInput)
	}
	if projection.LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want %q", projection.LastKind, terminal.TerminalKindQuery)
	}
	if projection.RowCount != 3 {
		t.Fatalf("row count = %d, want 3", projection.RowCount)
	}
	if projection.LastStatus != "query executed" {
		t.Fatalf("status = %q, want query executed", projection.LastStatus)
	}
	if projection.SQLText != "SELECT * FROM products" {
		t.Fatalf("sql text = %q, want %q", projection.SQLText, "SELECT * FROM products")
	}
	if projection.OutputText == "" {
		t.Fatalf("expected projection output text")
	}
	if projection.ResultColumns[0] != "id" || projection.ResultColumns[1] != "name" {
		t.Fatalf("result columns = %#v, want [id name]", projection.ResultColumns)
	}
	if projection.ResultRows[0] == "" || projection.ResultRows[0] != "1\tA" {
		t.Fatalf("first row = %q, want %q", projection.ResultRows[0], "1\tA")
	}
}

func TestShellCommandBusProjectsTerminalExecutionMetadata(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:                 terminal.TerminalKindQuery,
			SQLText:              "SELECT * FROM customers WHERE status = ?",
			Parameters:           []any{"active"},
			RowCount:             4,
			DurationMilliseconds: 17,
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: "SELECT * FROM customers WHERE status = ?",
	}); err != nil {
		t.Fatalf("dispatch terminal action: %v", err)
	}

	projection := state.Projection()
	if projection.ResultParameters == nil || len(projection.ResultParameters) != 1 || projection.ResultParameters[0] != "active" {
		t.Fatalf("result parameters = %#v", projection.ResultParameters)
	}
	if projection.ResultEstimate == "" || !strings.Contains(projection.ResultEstimate, "rows=4") {
		t.Fatalf("result estimate = %q", projection.ResultEstimate)
	}
	if projection.OutputText == "" {
		t.Fatal("expected output text")
	}
	if !strings.Contains(projection.OutputText, "parameters: active") {
		t.Fatalf("output text = %q", projection.OutputText)
	}
	if !strings.Contains(projection.OutputText, "duration: 17ms") {
		t.Fatalf("output text = %q", projection.OutputText)
	}
}

func TestShellCommandBusRunCanvasProjectsStatusAndMetadata(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:                 terminal.TerminalKindQuery,
			SQLText:              `SELECT "c"."id" FROM "customers" "c"`,
			Parameters:           []any{int64(10)},
			RowCount:             2,
			DurationMilliseconds: 22,
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionRunCanvas}); err != nil {
		t.Fatalf("run canvas: %v", err)
	}

	projection := state.Projection()
	if projection.LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want %q", projection.LastKind, terminal.TerminalKindQuery)
	}
	if !strings.Contains(projection.CanvasStatus, "canvas sales executed (2 rows, 22ms)") {
		t.Fatalf("canvas status = %q", projection.CanvasStatus)
	}
	if len(projection.ResultParameters) != 1 || projection.ResultParameters[0] != "10" {
		t.Fatalf("canvas result parameters = %#v", projection.ResultParameters)
	}
}

func TestShellCommandBusRunCanvasFailureMapsErrorToNodeStatus(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:      terminal.TerminalKindError,
			ErrorText: `Catalog Error: Table with name not found: "ghost"`,
			RowCount:  0,
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"bad", "kind":"table","table":"ghost","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionRunCanvas}); err != nil {
		t.Fatalf("run canvas: %v", err)
	}

	projection := state.Projection()
	if projection.CanvasStatus == "" || !strings.Contains(projection.CanvasStatus, "execution failed at node bad") {
		t.Fatalf("canvas status = %q", projection.CanvasStatus)
	}
}

func TestShellCommandBusProxyAndReportsTerminalErrors(t *testing.T) {
	fail := errors.New("terminal failed")
	runner := &fakeTerminalRunner{
		err: fail,
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	err := bus.Dispatch(Action{Kind: ActionRunTerminal, Payload: "bad sql"})
	if err == nil {
		t.Fatal("expected terminal failure to be returned")
	}
	if err != fail {
		t.Fatalf("err = %v, want %v", err, fail)
	}

	projection := state.Projection()
	if projection.LastInput != "" {
		t.Fatalf("last input = %q, want empty", projection.LastInput)
	}
	if projection.LastStatus != "idle" {
		t.Fatalf("status = %q, want idle", projection.LastStatus)
	}
}

func TestShellProjectionIncludesCatalogExplorerData(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		tables:   []string{"customers", "orders"},
		views:    []string{"order_view"},
		canvases: []string{"sales-canvas"},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)

	projection := state.Projection()
	if got, want := len(projection.CatalogTables), 2; got != want {
		t.Fatalf("catalog tables = %d, want %d", got, want)
	}
	if got, want := len(projection.CatalogViews), 1; got != want {
		t.Fatalf("catalog views = %d, want %d", got, want)
	}
	if got, want := len(projection.CatalogCanvases), 1; got != want {
		t.Fatalf("catalog canvases = %d, want %d", got, want)
	}
	if projection.CatalogTables[0] != "customers" {
		t.Fatalf("first table = %q, want %q", projection.CatalogTables[0], "customers")
	}
}

func TestShellCommandBusDispatchesShortcutToConsoleAndKeepsProjectionInSync(t *testing.T) {
	console := terminal.NewEventConsole(10)
	state := NewShellState(console)
	bus := NewShellCommandBus(nil, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionShortcut,
		Payload: "F12",
	}); err != nil {
		t.Fatalf("dispatch shortcut: %v", err)
	}
	if !state.Projection().ConsoleVisible {
		t.Fatal("expected projected console visibility true after shortcut")
	}

	if console.IsVisible() != state.Projection().ConsoleVisible {
		t.Fatal("projection and console visibility should be aligned")
	}
}

func TestShellCommandBusShortcutEscapeHidesConsole(t *testing.T) {
	console := terminal.NewEventConsole(10)
	state := NewShellState(console)
	bus := NewShellCommandBus(nil, state)

	if err := bus.Dispatch(Action{Kind: ActionShortcut, Payload: "F12"}); err != nil {
		t.Fatalf("dispatch f12: %v", err)
	}
	if !state.IsConsoleVisible() {
		t.Fatal("expected console visible after f12")
	}

	if err := bus.Dispatch(Action{Kind: ActionShortcut, Payload: "Escape"}); err != nil {
		t.Fatalf("dispatch escape: %v", err)
	}
	if state.IsConsoleVisible() {
		t.Fatal("expected console hidden after escape")
	}
}

func TestShellCommandBusRejectsUnsupportedAction(t *testing.T) {
	bus := NewShellCommandBus(nil, NewShellState(nil))
	err := bus.Dispatch(Action{Kind: "unsupported"})
	if err == nil {
		t.Fatal("expected unsupported action error")
	}
}

func TestShellCommandBusSetsConsoleStateExplicitly(t *testing.T) {
	state := NewShellState(terminal.NewEventConsole(10))
	bus := NewShellCommandBus(nil, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionSetConsoleState,
		Payload: "true",
	}); err != nil {
		t.Fatalf("dispatch set console state: %v", err)
	}
	if !state.IsConsoleVisible() {
		t.Fatal("expected console to be visible when set true")
	}
	if !state.console.IsVisible() {
		t.Fatal("expected underlying console to be visible when set true")
	}

	if err := bus.Dispatch(Action{
		Kind:    ActionSetConsoleState,
		Payload: "0",
	}); err != nil {
		t.Fatalf("dispatch set console state 0: %v", err)
	}
	if state.IsConsoleVisible() {
		t.Fatal("expected console to be hidden when set to 0")
	}
	if state.console.IsVisible() {
		t.Fatal("expected underlying console to be hidden when set to 0")
	}
}

func TestShellCommandBusRejectsUnknownShortcut(t *testing.T) {
	bus := NewShellCommandBus(nil, NewShellState(terminal.NewEventConsole(10)))
	err := bus.Dispatch(Action{Kind: ActionShortcut, Payload: "F11"})
	if err == nil {
		t.Fatal("expected unknown shortcut error")
	}
}

func TestShellCommandBusWithoutConsoleIgnoresShortcuts(t *testing.T) {
	bus := NewShellCommandBus(nil, NewShellState(nil))
	if err := bus.Dispatch(Action{Kind: ActionShortcut, Payload: "F12"}); err != nil {
		t.Fatalf("expected no-op when no console bound: %v", err)
	}
}

func TestShellStateCanSelectCanvasFromCatalogExplorer(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"name":  "ignore",
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	if err := state.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	projection := state.Projection()
	if projection.ActiveCanvas != "sales" {
		t.Fatalf("active canvas = %q, want sales", projection.ActiveCanvas)
	}
	if projection.CanvasDraftSpec == "" {
		t.Fatal("expected canvas draft spec")
	}
	if projection.CanvasStatus != "canvas loaded" {
		t.Fatalf("canvas status = %q, want canvas loaded", projection.CanvasStatus)
	}
}

func TestShellStateTracksCanvasDraftDirtyAndValidation(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	if err := state.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := state.SetCanvasDraft(`{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"name"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set canvas draft: %v", err)
	}
	projection := state.Projection()
	if !projection.CanvasDirty {
		t.Fatal("expected dirty canvas after draft edit")
	}
	if projection.CanvasValidation != "" {
		t.Fatalf("expected no validation error for valid draft, got %q", projection.CanvasValidation)
	}
	if projection.CanvasStatus != "canvas draft updated" {
		t.Fatalf("status = %q, want canvas draft updated", projection.CanvasStatus)
	}
}

func TestShellStateRevertsCanvasDraftAndClearsDirty(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	_ = state.SetActiveCanvas("sales")
	_ = state.SetCanvasDraft(`{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"name"}]}],"edges":[]}`)
	if err := state.RevertCanvasDraft(); err != nil {
		t.Fatalf("revert draft: %v", err)
	}
	projection := state.Projection()
	if projection.CanvasDirty {
		t.Fatal("expected dirty to be false after revert")
	}
	if !strings.Contains(projection.CanvasStatus, "reverted") {
		t.Fatalf("canvas status = %q, want contains reverted", projection.CanvasStatus)
	}
}

func TestShellCommandBusSavesCanvasDraftThroughTerminalAndCommits(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "saved canvas sales",
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionSetCanvasDraft, Payload: `{"nodes":[{"id":"c","kind":"table","table":"customers","fields":[{"name":"name"}]}],"edges":[]}`}); err != nil {
		t.Fatalf("set draft: %v", err)
	}
	if runner.calls != 0 {
		t.Fatal("runner should not be called before save")
	}

	if err := bus.Dispatch(Action{Kind: ActionSaveCanvas}); err != nil {
		t.Fatalf("save canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, "\\canvas save sales '{\"version\":\"1.0.0\",") {
		t.Fatalf("runner input = %q, want quoted save command with normalized spec", runner.result.Input)
	}
	if projection := state.Projection(); projection.CanvasDirty {
		t.Fatal("expected dirty to clear after save")
	}
}

func TestShellCommandBusTerminalSaveCommitForActiveCanvas(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "saved canvas sales",
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionSetCanvasDraft,
		Payload: `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"created_at"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("set draft: %v", err)
	}

	if err := bus.Dispatch(Action{Kind: ActionRunTerminal, Payload: `\canvas save sales '{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"created_at"}]}],"edges":[]}'`}); err != nil {
		t.Fatalf("run terminal save: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if got := state.Projection().CanvasDirty; got {
		t.Fatal("expected dirty to clear after terminal save command")
	}
	if strings.Contains(state.Projection().CanvasDraftSpec, "created_at") == false {
		t.Fatalf("draft should align with saved spec; got %q", state.Projection().CanvasDraftSpec)
	}
}

func TestShellCommandBusTerminalDeleteActiveCanvasClearsSelection(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted canvas sales",
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}

	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: `\canvas delete sales`,
	}); err != nil {
		t.Fatalf("run terminal delete: %v", err)
	}
	if state.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want cleared", state.Projection().ActiveCanvas)
	}
}

func TestShellCommandBusTerminalRenameActiveCanvasUpdatesSelection(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed canvas sales -> sales_yearly",
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales", "sales_yearly"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}

	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: `\canvas rename sales sales_yearly`,
	}); err != nil {
		t.Fatalf("run terminal rename: %v", err)
	}
	if state.Projection().ActiveCanvas != "sales_yearly" {
		t.Fatalf("active canvas = %q, want sales_yearly", state.Projection().ActiveCanvas)
	}
}

func TestShellCommandBusMoveCanvasNodeMutatesActiveCanvasDraft(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}],"x":10,"y":5},{"id":"orders","kind":"table","table":"orders","fields":[{"name":"id"}],"x":2,"y":2}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}

	payload, err := json.Marshal(map[string]any{"node_id": "customers", "x": -25, "y": 12.5})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionMoveCanvasNode,
		Payload: string(payload),
	}); err != nil {
		t.Fatalf("move canvas node: %v", err)
	}

	projection := state.Projection()
	if !projection.CanvasDirty {
		t.Fatal("expected canvas draft dirty after move")
	}
	spec, err := query.ParseCanvasSpec([]byte(projection.CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if got, want := spec.Nodes[0].X, 0.0; got != want {
		t.Fatalf("node x = %v, want %v", got, want)
	}
	if got, want := spec.Nodes[0].Y, 12.5; got != want {
		t.Fatalf("node y = %v, want %v", got, want)
	}
}

func TestShellCommandBusSetCanvasNodeFieldsMutatesDraftAndFiltersUnknowns(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}],"x":0,"y":0}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"node_id": "customers",
		"fields":  []string{"name", "missing", "name"},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionSetNodeFields,
		Payload: string(payload),
	}); err != nil {
		t.Fatalf("set node fields: %v", err)
	}

	spec, err := query.ParseCanvasSpec([]byte(state.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Nodes[0].SelectedFields) != 1 {
		t.Fatalf("selected fields = %#v, want [name]", spec.Nodes[0].SelectedFields)
	}
	if got := spec.Nodes[0].SelectedFields[0]; got != "name" {
		t.Fatalf("selected field = %q, want %q", got, "name")
	}
}

func TestShellCommandBusAddPatchDeleteCanvasEdgeMutatesDraft(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"region"}]},{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"},{"name":"total"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}

	addPayload, err := json.Marshal(map[string]any{
		"id":         "join-1",
		"kind":       "join",
		"from":       "customers",
		"to":         "orders",
		"joinType":   "LEFT",
		"fromColumn": "id",
		"toColumn":   "customer_id",
	})
	if err != nil {
		t.Fatalf("marshal add payload: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionAddCanvasEdge,
		Payload: string(addPayload),
	}); err != nil {
		t.Fatalf("add canvas edge: %v", err)
	}

	patchPayload, err := json.Marshal(map[string]any{
		"id":         "join-1",
		"kind":       "join",
		"from":       "customers",
		"to":         "orders",
		"joinType":   "INNER",
		"fromColumn": "region",
		"toColumn":   "total",
	})
	if err != nil {
		t.Fatalf("marshal patch payload: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionPatchCanvasEdge,
		Payload: string(patchPayload),
	}); err != nil {
		t.Fatalf("patch canvas edge: %v", err)
	}

	deletePayload, err := json.Marshal(map[string]any{"edge_id": "join-1"})
	if err != nil {
		t.Fatalf("marshal delete payload: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionDeleteCanvasEdge,
		Payload: string(deletePayload),
	}); err != nil {
		t.Fatalf("delete canvas edge: %v", err)
	}

	spec, err := query.ParseCanvasSpec([]byte(state.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft spec: %v", err)
	}
	if len(spec.Edges) != 0 {
		t.Fatalf("edges = %d, want 0", len(spec.Edges))
	}
}

func TestShellCommandBusAddCanvasNodeMutatesDraft(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"name"}]},{"id":"orders","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}

	node := query.CanvasNode{
		ID:     "shipments",
		Kind:   query.CanvasNodeKindTable,
		Table:  "shipments",
		Fields: []query.CanvasField{{Name: "id"}},
	}
	payload, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal node payload: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasNode, Payload: string(payload)}); err != nil {
		t.Fatalf("add canvas node: %v", err)
	}

	spec, err := query.ParseCanvasSpec([]byte(state.Projection().CanvasDraftSpec))
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if len(spec.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(spec.Nodes))
	}
	if !state.Projection().CanvasDirty {
		t.Fatal("expected canvas dirty after add")
	}
}

func TestShellCommandBusRejectsAddCanvasNodeWithoutActiveCanvas(t *testing.T) {
	state := NewShellState(nil)
	bus := NewShellCommandBus(nil, state)
	payload, err := json.Marshal(query.CanvasNode{
		ID:    "n1",
		Table: "customers",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasNode, Payload: string(payload)}); err == nil {
		t.Fatal("expected add canvas node without active canvas to fail")
	}
}

func TestShellCommandBusRejectsCanvasNodePayloadWithoutID(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	payload := `{"table":"customers","fields":[{"name":"id"}]}`
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasNode, Payload: payload}); err == nil {
		t.Fatal("expected missing id node payload to fail")
	}
}

func TestShellCommandBusRejectsInvalidCanvasNodePayloadJSON(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasNode, Payload: "{invalid json"}); err == nil {
		t.Fatal("expected invalid node payload json to fail")
	}
}

func TestShellCommandBusRejectsAddCanvasEdgeWithoutActiveCanvas(t *testing.T) {
	state := NewShellState(nil)
	bus := NewShellCommandBus(nil, state)
	payload := query.CanvasEdge{
		ID:         "join-1",
		FromNode:   "customers",
		ToNode:     "orders",
		FromColumn: "id",
		ToColumn:   "customer_id",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasEdge, Payload: string(raw)}); err == nil {
		t.Fatal("expected add canvas edge without active canvas to fail")
	}
}

func TestShellCommandBusRejectsCanvasEdgePayloadWithoutID(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"}]}
			],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	payload := `{"from":"customers","to":"orders","fromColumn":"id","toColumn":"customer_id","joinType":"INNER"}`
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasEdge, Payload: payload}); err == nil {
		t.Fatal("expected missing edge id payload to fail")
	}
}

func TestShellCommandBusRejectsInvalidCanvasEdgePayloadJSON(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"}]}
			],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionAddCanvasEdge, Payload: "{invalid json"}); err == nil {
		t.Fatal("expected invalid edge payload json to fail")
	}
}

func TestShellCommandBusRejectsDeleteCanvasEdgePayloadWithoutID(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[
				{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]},
				{"id":"orders","kind":"table","table":"orders","fields":[{"name":"customer_id"}]}
			],"edges":[
				{"id":"join-1","kind":"join","from":"customers","to":"orders","fromColumn":"id","toColumn":"customer_id","joinType":"INNER"}
			]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(nil, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	payload := `{}`
	if err := bus.Dispatch(Action{Kind: ActionDeleteCanvasEdge, Payload: payload}); err == nil {
		t.Fatal("expected missing edge id delete payload to fail")
	}
}

func TestShellCommandBusTerminalCanvasNewRefreshesCatalogProjection(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "state-canvas-new.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepository := catalog.NewCanvasRepository(database.SQL)
	explorer := &catalogShellStateExplorer{repository: canvasRepository}
	runner := terminal.NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepository)
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: "\\canvas new stage-overview",
	}); err != nil {
		t.Fatalf("run terminal command: %v", err)
	}
	projection := state.Projection()
	if len(projection.CatalogCanvases) != 1 {
		t.Fatalf("catalog canvas count = %d, want 1", len(projection.CatalogCanvases))
	}
	if projection.CatalogCanvases[0] != "stage-overview" {
		t.Fatalf("canvas name = %q, want stage-overview", projection.CatalogCanvases[0])
	}
	if projection.ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", projection.ActiveCanvas)
	}
	if projection.CanvasStatus != "canvas created: stage-overview" {
		t.Fatalf("canvas status = %q, want canvas created: stage-overview", projection.CanvasStatus)
	}
}

func TestShellCommandBusCanvasNewActionUpdatesCanvasStatus(t *testing.T) {
	runner := &fakeTerminalRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas sales-overview",
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasNew,
		Payload: "sales-overview",
	}); err != nil {
		t.Fatalf("canvas new action: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if projection := state.Projection(); projection.CanvasStatus != "canvas created: sales-overview" {
		t.Fatalf("canvas status = %q, want canvas created: sales-overview", projection.CanvasStatus)
	}
}

func TestShellCommandBusTerminalCanvasDeleteRemovesCanvasFromProjectionCatalog(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "state-canvas-delete.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepository := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepository.Create(catalog.Canvas{
		ID:       "canvas-sales",
		Name:     "sales",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	explorer := &catalogShellStateExplorer{repository: canvasRepository}
	runner := terminal.NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepository)
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: `\canvas delete sales`,
	}); err != nil {
		t.Fatalf("run terminal command: %v", err)
	}

	projection := state.Projection()
	if projection.ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", projection.ActiveCanvas)
	}
	for _, name := range projection.CatalogCanvases {
		if name == "sales" {
			t.Fatal("expected deleted canvas not to appear in projection catalog list")
		}
	}
}

func TestShellCommandBusTerminalCanvasRenameRefreshesCatalogProjection(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "state-canvas-rename.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepository := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepository.Create(catalog.Canvas{
		ID:       "canvas-sales",
		Name:     "sales",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	explorer := &catalogShellStateExplorer{repository: canvasRepository}
	runner := terminal.NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepository)
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionRunTerminal,
		Payload: `\canvas rename sales sales-yearly`,
	}); err != nil {
		t.Fatalf("run terminal command: %v", err)
	}

	projection := state.Projection()
	if projection.ActiveCanvas != "sales-yearly" {
		t.Fatalf("active canvas = %q, want sales-yearly", projection.ActiveCanvas)
	}
	var names []string
	for _, name := range projection.CatalogCanvases {
		names = append(names, name)
		if name == "sales" {
			t.Fatal("expected renamed canvas name not to appear in projection catalog list")
		}
	}
	if len(names) != 1 || names[0] != "sales-yearly" {
		t.Fatalf("catalog canvases = %#v, want [sales-yearly]", names)
	}
}

type catalogShellStateExplorer struct {
	repository *catalog.CanvasRepository
}

func (e *catalogShellStateExplorer) ListTables() ([]string, error) {
	return nil, nil
}

func (e *catalogShellStateExplorer) ListViews() ([]string, error) {
	return nil, nil
}

func (e *catalogShellStateExplorer) ListCanvases() ([]string, error) {
	canvases, err := e.repository.List()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, canvas := range canvases {
		names = append(names, canvas.Name)
	}
	return names, nil
}

func (e *catalogShellStateExplorer) LoadCanvasSpec(name string) (string, error) {
	canvas, err := e.repository.FindByName(name)
	if err != nil {
		return "", err
	}
	return canvas.SpecJSON, nil
}
