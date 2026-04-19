package appstate

import (
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestStateTransitionToggleConsoleVisibility(t *testing.T) {
	state := NewShellState(terminal.NewEventConsole(10))

	if state.IsConsoleVisible() {
		t.Fatal("expected console hidden by default")
	}
	if err := NewShellCommandBus(nil, state).Dispatch(Action{Kind: ActionToggleConsole}); err != nil {
		t.Fatalf("dispatch first toggle: %v", err)
	}
	if !state.IsConsoleVisible() {
		t.Fatal("expected console visible after transition")
	}

	state.SetConsoleVisible(false)
	if state.IsConsoleVisible() {
		t.Fatal("expected console hidden after set console visible false")
	}
}

func TestStateTransitionDefaultsOnNilState(t *testing.T) {
	if state := (*ShellState)(nil); state.IsConsoleVisible() != false {
		t.Fatal("expected nil state to read as not visible")
	}
	if got := state_projectionForNil().LastStatus; got != "idle" {
		t.Fatalf("projection status = %q, want idle", got)
	}
}

func state_projectionForNil() ShellProjection {
	return (*ShellState)(nil).Projection()
}
