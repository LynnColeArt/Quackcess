package shell

import (
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/appstate"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestCanvasPanelShortcutCtrlSPersistsCanvasDraft(t *testing.T) {
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
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(runner, state)), nil)

	if err := presenter.model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.model.SetCanvasDraft(`{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set draft: %v", err)
	}
	if err := presenter.HandleShortcut("Ctrl+S"); err != nil {
		t.Fatalf("handle Ctrl+S: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
}

func TestCanvasPanelShortcutCtrlRRevertsDraftState(t *testing.T) {
	explorer := &fakeShellCatalogExplorer{
		canvases: map[string]string{
			"sales": `{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"}]}],"edges":[]}`,
		},
	}
	state := appstate.NewShellStateWithCatalogExplorer(terminal.NewEventConsole(10), explorer)
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(nil, state)), nil)

	if err := presenter.model.SetActiveCanvas("sales"); err != nil {
		t.Fatalf("set active canvas: %v", err)
	}
	if err := presenter.model.SetCanvasDraft(`{"nodes":[{"id":"s","kind":"table","table":"orders","fields":[{"name":"id"},{"name":"created_at"}]}],"edges":[]}`); err != nil {
		t.Fatalf("set draft: %v", err)
	}
	if err := presenter.HandleShortcut("Ctrl+R"); err != nil {
		t.Fatalf("handle Ctrl+R: %v", err)
	}
	if got := presenter.Projection().CanvasDirty; got {
		t.Fatal("expected canvas dirty to clear on Ctrl+R")
	}
}

func TestCanvasPanelShortcutDelegatesUnknownShortcutToStateBus(t *testing.T) {
	state := appstate.NewShellState(terminal.NewEventConsole(10))
	presenter := NewShellPresenter(NewShellModel(appstate.NewShellCommandBus(nil, state)), nil)

	if err := presenter.HandleShortcut("F11"); err == nil {
		t.Fatal("expected unknown shortcut to fail")
	}
}
