package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPermissionMatrixSupportsPrincipalRules(t *testing.T) {
	tmp := t.TempDir()
	matrixPath := filepath.Join(tmp, "matrix.json")
	matrix := `{
		"defaultAllow": false,
		"principals": {
			"alice": ["query.execute", "schema.inspect"],
			"analytics": ["*"],
			"*": ["artifact.get"]
		}
	}`
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o644); err != nil {
		t.Fatalf("write matrix: %v", err)
	}

	authz, err := LoadPermissionMatrix(matrixPath)
	if err != nil {
		t.Fatalf("load matrix: %v", err)
	}

	if !authz.CanAccess("alice", "query.execute") {
		t.Fatal("alice should access query.execute")
	}
	if !authz.CanAccess("alice", "schema.inspect") {
		t.Fatal("alice should access schema.inspect")
	}
	if authz.CanAccess("alice", "vector.rebuild") {
		t.Fatal("alice should not access vector.rebuild")
	}
	if !authz.CanAccess("analytics", "system.ping") {
		t.Fatal("analytics wildcard should access all tools")
	}
	if !authz.CanAccess("alice", "artifact.get") {
		t.Fatal("wildcard principal rule should grant artifact.get")
	}
}

func TestPermissionMatrixDefaultsWhenConfiguredTrue(t *testing.T) {
	data := []byte(`{"defaultAllow": true}`)
	authz, err := ParsePermissionMatrix(data)
	if err != nil {
		t.Fatalf("parse matrix: %v", err)
	}
	if !authz.CanAccess("random", "anything") {
		t.Fatal("defaultAllow=true should permit unknown principal")
	}
}

func TestPermissionMatrixRejectsInvalidMatrixShape(t *testing.T) {
	raw := []byte(`{"defaultAllow":false,"principals":{"alice":"query.execute"}}`)
	authz, err := ParsePermissionMatrix(raw)
	if authz != nil {
		t.Fatal("expected nil authorizer on invalid matrix")
	}
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "decode permission matrix") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPermissionMatrixLoadRejectsMissingFile(t *testing.T) {
	if _, err := LoadPermissionMatrix(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected load error")
	}
}

func TestServerPublishesUnauthorizedEventForDeniedTools(t *testing.T) {
	authz := NewAllowlistAuthorizer(false)
	server := NewServer(authz, NewEventBus())
	if err := RegisterCoreTools(server, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	eventsCh, cancel := server.events.Subscribe(4)
	t.Cleanup(cancel)

	result := server.CallTool(context.Background(), &CallRequest{
		Tool:      "query.execute",
		Principal: "alice",
	})
	if result.Error == nil || result.Error.Code != ErrorCodeUnauthorized {
		t.Fatalf("error = %v, want unauthorized", result.Error)
	}

	events := collectEventsFrom(eventsCh, t, 1)
	if got := events[0].Type; got != "mcp.call.denied" {
		t.Fatalf("event type = %q, want mcp.call.denied", got)
	}
	if events[0].Tool != "query.execute" {
		t.Fatalf("event tool = %q, want query.execute", events[0].Tool)
	}
}
