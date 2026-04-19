//go:build gtk3
// +build gtk3

package gtk

import (
	"strings"
	"testing"
)

func TestExplorerCommandForSelectionBuildsReadableSelect(t *testing.T) {
	command, err := explorerCommandForSelection("table", "orders")
	if err != nil {
		t.Fatalf("explorer command: %v", err)
	}
	if got, want := strings.TrimSpace(command), `SELECT * FROM "orders"`; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}

	command, err = explorerCommandForSelection("view", "recent_orders")
	if err != nil {
		t.Fatalf("explorer command: %v", err)
	}
	if got, want := strings.TrimSpace(command), `SELECT * FROM "recent_orders"`; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}

	command, err = explorerCommandForSelection("canvas", "sales canvas")
	if err != nil {
		t.Fatalf("explorer command: %v", err)
	}
	if got, want := strings.TrimSpace(command), `\canvas sales canvas`; got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestExplorerCommandForSelectionRejectsUnsupportedSelection(t *testing.T) {
	if _, err := explorerCommandForSelection("unknown", "x"); err == nil {
		t.Fatal("expected unknown selection to fail")
	}
	if _, err := explorerCommandForSelection("table", ""); err == nil {
		t.Fatal("expected empty name to fail")
	}
}
