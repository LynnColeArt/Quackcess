package shell

import (
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestShellCanvasToolbarSaveActionBuildsCanvasSaveCommand(t *testing.T) {
	runner := &shellFakeRunner{
		result: terminal.TerminalResult{
			Kind:    terminal.TerminalKindHelp,
			Message: "saved canvas sales",
		},
	}
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	_ = model.SetCanvasDraft(`{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}`)

	if err := model.SaveActiveCanvas(); err != nil {
		t.Fatalf("save active canvas: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	if !strings.HasPrefix(runner.last, "\\canvas save sales '") {
		t.Fatalf("runner input = %q, want canvas save command", runner.last)
	}
	if !strings.HasSuffix(runner.last, "'") {
		t.Fatalf("runner input = %q, expected trailing quote", runner.last)
	}
}

func TestShellCanvasToolbarRejectsSaveWithoutActiveCanvas(t *testing.T) {
	runner := &shellFakeRunner{
		result: terminal.TerminalResult{
			Kind: terminal.TerminalKindHelp,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), &fakeShellCatalogExplorer{
		canvases: map[string]string{},
	})
	model := NewShellModel(appstate.NewShellCommandBus(runner, state))

	if err := model.SaveActiveCanvas(); err == nil {
		t.Fatal("expected save without active canvas to fail")
	}
}

func TestShellCanvasToolbarClearResetsActiveCanvasState(t *testing.T) {
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
	if err := model.ClearCanvas(); err != nil {
		t.Fatalf("clear canvas: %v", err)
	}
	if got := model.Projection().ActiveCanvas; got != "" {
		t.Fatalf("active canvas = %q, want empty", got)
	}
}
