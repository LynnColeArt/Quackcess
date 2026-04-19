package mcp

import (
	"context"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestServerDefaultsToDenyWithNilAuthorizer(t *testing.T) {
	server := NewServer(nil, nil)
	if err := RegisterCoreTools(server, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool:      "system.ping",
		Principal: "alice",
	})
	if result.Error == nil || result.Error.Code != ErrorCodeUnauthorized {
		t.Fatalf("error code = %v, want unauthorized", result.Error)
	}
}

func TestAllowlistSupportsWildcardAndToolSpecificGrants(t *testing.T) {
	authz := NewAllowlistAuthorizer(false)
	queryRunner := &fakeQueryRunner{
		result: terminal.TerminalResult{
			Kind: terminal.TerminalKindQuery,
		},
	}
	authz.Grant("alice", "query.execute")
	authz.Grant("analytics", "*")

	server := NewServer(authz, nil)
	if err := RegisterCoreTools(server, CoreTools{
		CatalogService: &fakeCatalogService{
			tables:   []string{"alpha"},
			views:    []string{},
			canvases: []string{},
		},
		QueryRunner: queryRunner,
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	if err := catalogCheck(server, "schema.inspect", "alice", nil); err == nil {
		t.Fatalf("alice should not access schema.inspect")
	}
	if err := catalogCheck(server, "query.execute", "alice", []byte(`{"sql":"SELECT 1"}`)); err != nil {
		t.Fatalf("alice should access query.execute: %v", err)
	}
	if err := catalogCheck(server, "schema.inspect", "analytics", nil); err != nil {
		t.Fatalf("analytics wildcard access: %v", err)
	}
}

func catalogCheck(server *Server, tool string, principal string, args []byte) error {
	result := server.CallTool(context.Background(), &CallRequest{
		Tool:      tool,
		Principal: principal,
		Args:      args,
	})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func TestAllowlistAuthorizerCloneIsIndependentOfSource(t *testing.T) {
	authorizer := NewAllowlistAuthorizer(false)
	authorizer.Grant("alice", "schema.inspect")
	copy := authorizer.Clone()

	if copy == nil {
		t.Fatal("expected cloned authorizer")
	}
	if !copy.CanAccess("alice", "schema.inspect") {
		t.Fatal("expected clone to carry copied grants")
	}
	copy.Grant("alice", "query.execute")
	if authorizer.CanAccess("alice", "query.execute") {
		t.Fatal("expected clone mutation not to affect source")
	}
}
