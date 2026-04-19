package terminal

import (
	"testing"
	"time"
)

func TestEventConsoleToggleByF12(t *testing.T) {
	console := NewEventConsole(5)
	if console.IsVisible() {
		t.Fatal("expected hidden by default")
	}
	if !console.HandleShortcut("F12") {
		t.Fatal("expected F12 to be handled")
	}
	if !console.IsVisible() {
		t.Fatal("expected visible after F12")
	}
	if !console.HandleShortcut("f12") {
		t.Fatal("expected f12 to be handled")
	}
	if console.IsVisible() {
		t.Fatal("expected hidden after second toggle")
	}
}

func TestEventConsoleIgnoresUnknownShortcut(t *testing.T) {
	console := NewEventConsole(5)
	if console.HandleShortcut("F11") {
		t.Fatal("expected F11 to be ignored")
	}
	if console.IsVisible() {
		t.Fatal("expected visibility unchanged")
	}
}

func TestEventConsoleEvictsOldestWhenOverflow(t *testing.T) {
	console := NewEventConsole(2)
	console.Append("query.executed", "terminal", "first")
	console.Append("query.failed", "terminal", "second")
	console.Append("query.executed", "terminal", "third")

	items := console.Items()
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Message != "second" || items[1].Message != "third" {
		t.Fatalf("unexpected items ordering: %#v", items)
	}
}

func TestEventConsoleAppendEventDefaultsTimestamp(t *testing.T) {
	console := NewEventConsole(5)
	now := time.Now().Add(-time.Minute)
	console.AppendEvent(ConsoleEvent{
		Timestamp: now.Add(-time.Minute),
		Kind:      "query.executed",
		Source:    "terminal",
		Message:   "custom timestamp",
	})
	console.Append("query.executed", "terminal", "auto timestamp")
	items := console.Items()
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if !items[0].Timestamp.After(time.Time{}) {
		t.Fatal("missing timestamp for first event")
	}
	if !items[1].Timestamp.After(items[0].Timestamp) {
		t.Fatal("expected latest event timestamp to be greater than prior item")
	}
}
