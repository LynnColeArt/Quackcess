package appstate

import (
	"errors"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type commandBusFakeRunner struct {
	calls  int
	result terminal.TerminalResult
	err    error
}

func (f *commandBusFakeRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	f.calls++
	if f.err != nil {
		return terminal.TerminalResult{}, f.err
	}
	f.result.Input = input
	return f.result, nil
}

func TestCommandBusDispatchRoutesRunTerminalActions(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:     terminal.TerminalKindQuery,
			RowCount: 1,
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionRunTerminal, Payload: "SELECT 1"}); err != nil {
		t.Fatalf("dispatch run terminal: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	if state.Projection().LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want query", state.Projection().LastKind)
	}
}

func TestCommandBusDispatchReturnsUnsupportedActionError(t *testing.T) {
	bus := NewShellCommandBus(nil, NewShellState(nil))
	if err := bus.Dispatch(Action{Kind: "unsupported"}); err == nil {
		t.Fatal("expected unsupported action to fail")
	}
}

func TestCommandBusDispatchSurfaceTerminalFailures(t *testing.T) {
	fail := errors.New("failed command")
	runner := &commandBusFakeRunner{err: fail}
	bus := NewShellCommandBus(runner, NewShellState(nil))

	err := bus.Dispatch(Action{Kind: ActionRunTerminal, Payload: "SELECT bad"})
	if err != fail {
		t.Fatalf("dispatch error = %v, want %v", err, fail)
	}
}

func TestCommandBusDispatchRunsActiveCanvasSQL(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:      terminal.TerminalKindQuery,
			SQLText:   "SELECT 1",
			RowCount:  2,
			ErrorText: "",
		},
	}
	explorer := &fakeCatalogExplorer{
		canvases: []string{"analytics"},
		specs: map[string]string{
			"analytics": `{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)

	spec, err := query.ParseCanvasSpec([]byte(`{"nodes":[{"id":"customers","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`))
	if err != nil {
		t.Fatalf("parse canvas spec: %v", err)
	}
	expected, err := query.GenerateSQLFromCanvas(spec)
	if err != nil {
		t.Fatalf("generate sql: %v", err)
	}

	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "analytics"}); err != nil {
		t.Fatalf("set canvas: %v", err)
	}
	if err := bus.Dispatch(Action{Kind: ActionRunCanvas}); err != nil {
		t.Fatalf("run canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.Contains(runner.result.Input, strings.TrimSpace(expected.SQL)) {
		t.Fatalf("last runner input = %q, want contains %q", runner.result.Input, expected.SQL)
	}
	if !strings.Contains(strings.ToUpper(runner.result.Input), "LIMIT ") {
		t.Fatalf("last runner input = %q, expected LIMIT clause for safe canvas preview", runner.result.Input)
	}
	if state.Projection().LastKind != terminal.TerminalKindQuery {
		t.Fatalf("last kind = %q, want %q", state.Projection().LastKind, terminal.TerminalKindQuery)
	}
}

func TestCommandBusDispatchRejectsRunCanvasWithoutActiveCanvas(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind: terminal.TerminalKindQuery,
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionRunCanvas}); err == nil {
		t.Fatal("expected run canvas without active canvas to fail")
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
}

func TestCommandBusDispatchesCanvasNewAction(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas project-overview",
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasNew,
		Payload: `{"name":"project-overview"}`,
	}); err != nil {
		t.Fatalf("dispatch canvas new: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if strings.TrimSpace(runner.result.Input) != `\canvas new project-overview` {
		t.Fatalf("last runner input = %q, want %q", runner.result.Input, `\canvas new project-overview`)
	}
}

func TestCommandBusDispatchesCanvasRenameAction(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed canvas sales -> sales-archive",
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasRename,
		Payload: `{"old_name":"sales","new_name":"sales-archive"}`,
	}); err != nil {
		t.Fatalf("dispatch canvas rename: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if state.Projection().ActiveCanvas != "sales-archive" {
		t.Fatalf("active canvas = %q, want sales-archive", state.Projection().ActiveCanvas)
	}
}

func TestCommandBusDispatchesCanvasDeleteAction(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"sales"},
		specs: map[string]string{
			"sales": `{"nodes":[{"id":"c","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted canvas sales",
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "sales"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasDelete,
		Payload: `{"name":"sales"}`,
	}); err != nil {
		t.Fatalf("dispatch canvas delete: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if state.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", state.Projection().ActiveCanvas)
	}
}

func TestCommandBusDispatchesCanvasNewActionWithRawPayload(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "created canvas finance-overview",
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasNew,
		Payload: "finance-overview",
	}); err != nil {
		t.Fatalf("dispatch canvas new: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	if strings.TrimSpace(runner.result.Input) != `\canvas new finance-overview` {
		t.Fatalf("last runner input = %q, want %q", runner.result.Input, `\canvas new finance-overview`)
	}
}

func TestCommandBusDispatchesCanvasRenameActionWithRawPayload(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"finance"},
		specs: map[string]string{
			"finance": `{"nodes":[{"id":"c","kind":"table","table":"accounts","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "renamed canvas finance -> finance-archive",
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "finance"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}

	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasRename,
		Payload: "finance finance-archive",
	}); err != nil {
		t.Fatalf("dispatch canvas rename: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	if state.Projection().ActiveCanvas != "finance-archive" {
		t.Fatalf("active canvas = %q, want finance-archive", state.Projection().ActiveCanvas)
	}
	if strings.TrimSpace(runner.result.Input) != `\canvas rename finance finance-archive` {
		t.Fatalf("last runner input = %q, want %q", runner.result.Input, `\canvas rename finance finance-archive`)
	}
}

func TestCommandBusDispatchesCanvasDeleteActionWithRawPayload(t *testing.T) {
	explorer := &fakeCatalogExplorer{
		canvases: []string{"finance"},
		specs: map[string]string{
			"finance": `{"nodes":[{"id":"c","kind":"table","table":"accounts","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "deleted finance",
		},
	}
	state := NewShellStateWithCatalogExplorer(nil, explorer)
	bus := NewShellCommandBus(runner, state)
	if err := bus.Dispatch(Action{Kind: ActionSetCanvas, Payload: "finance"}); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}

	if err := bus.Dispatch(Action{
		Kind:    ActionCanvasDelete,
		Payload: "finance",
	}); err != nil {
		t.Fatalf("dispatch canvas delete: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	if state.Projection().ActiveCanvas != "" {
		t.Fatalf("active canvas = %q, want empty", state.Projection().ActiveCanvas)
	}
	if strings.TrimSpace(runner.result.Input) != `\canvas delete finance` {
		t.Fatalf("last runner input = %q, want %q", runner.result.Input, `\canvas delete finance`)
	}
}

func TestCommandBusDispatchesCanvasActionWithoutRunner(t *testing.T) {
	state := NewShellState(nil)
	bus := NewShellCommandBus(nil, state)

	if err := bus.Dispatch(Action{Kind: ActionCanvasNew, Payload: `{"name":"sales"}`}); err == nil {
		t.Fatal("expected missing runner error for canvas new")
	}
	if err := bus.Dispatch(Action{Kind: ActionCanvasRename, Payload: `{"old_name":"a","new_name":"b"}`}); err == nil {
		t.Fatal("expected missing runner error for canvas rename")
	}
	if err := bus.Dispatch(Action{Kind: ActionCanvasDelete, Payload: `{"name":"sales"}`}); err == nil {
		t.Fatal("expected missing runner error for canvas delete")
	}
}

func TestCommandBusRejectsCanvasCommandPayloadErrors(t *testing.T) {
	runner := &commandBusFakeRunner{
		result: terminal.TerminalResult{
			Kind: terminal.TerminalKindHelp,
		},
	}
	state := NewShellState(nil)
	bus := NewShellCommandBus(runner, state)

	if err := bus.Dispatch(Action{Kind: ActionCanvasNew, Payload: ""}); err == nil {
		t.Fatal("expected canvas new payload error")
	}
	if err := bus.Dispatch(Action{Kind: ActionCanvasRename, Payload: "a"}); err == nil {
		t.Fatal("expected canvas rename payload error")
	}
	if err := bus.Dispatch(Action{Kind: ActionCanvasDelete, Payload: ""}); err == nil {
		t.Fatal("expected canvas delete payload error")
	}
}
