package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestCallToolPublishesCallLifecycleEvents(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), NewEventBus())
	if err := server.RegisterTool(ToolDefinition{
		Name: "system.ping",
		Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
			return map[string]any{"pong": true}, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	ch, cancel := server.events.Subscribe(8)
	t.Cleanup(cancel)
	result := server.CallTool(context.Background(), &CallRequest{Tool: "system.ping"})
	if result.Error != nil {
		t.Fatalf("ping result error: %v", result.Error)
	}
	events := collectEventsFrom(ch, t, 2, "mcp.call.started", "mcp.call.success")
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Type != "mcp.call.started" {
		t.Fatalf("first event = %q, want mcp.call.started", events[0].Type)
	}
	if events[1].Type != "mcp.call.success" {
		t.Fatalf("second event = %q, want mcp.call.success", events[1].Type)
	}
}

func TestCallToolPanicPublishesFailureEvent(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), NewEventBus())
	if err := server.RegisterTool(ToolDefinition{
		Name: "panic.tool",
		Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
			panic("tool panic")
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	ch, cancel := server.events.Subscribe(8)
	t.Cleanup(cancel)
	result := server.CallTool(context.Background(), &CallRequest{Tool: "panic.tool"})
	if result.Error == nil || result.Error.Code != ErrorCodeHandlerError {
		t.Fatalf("expected handler_error, got %v", result.Error)
	}
	events := collectEventsFrom(ch, t, 2, "mcp.call.started", "mcp.call.failure")
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[1].Type != "mcp.call.failure" {
		t.Fatalf("failure event = %q, want mcp.call.failure", events[1].Type)
	}
}

func TestEventBusSubscribeAndCancel(t *testing.T) {
	bus := NewEventBus()
	ch, cancel := bus.Subscribe(0)
	if cap(ch) != 16 {
		t.Fatalf("subscribe capacity = %d, want 16", cap(ch))
	}
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed after cancel")
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected channel close after cancel")
	}
}

func collectEventsFrom(ch <-chan Event, t *testing.T, expected int, expectedTypes ...string) []Event {
	t.Helper()

	got := make([]Event, 0, expected)
	for len(got) < expected {
		select {
		case event := <-ch:
			got = append(got, event)
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out waiting for events after collecting %#v", got)
		}
	}

	if len(expectedTypes) > 0 {
		for i, wantType := range expectedTypes {
			if got[i].Type != wantType {
				t.Fatalf("event[%d].type = %q, want %q", i, got[i].Type, wantType)
			}
		}
	}
	return got
}
