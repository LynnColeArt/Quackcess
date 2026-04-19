package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type alwaysFailRunner struct{}

func (r *alwaysFailRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	return terminal.TerminalResult{}, fmt.Errorf("query engine unavailable")
}

func TestCallToolReturnsMissingRequestWhenNilRequest(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), nil)
	if result.Error == nil || result.Error.Code != ErrorCodeMissingRequest {
		t.Fatalf("error = %v, want missing_request", result.Error)
	}
}

func TestCallToolReturnsMissingRequestWhenToolMissing(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{})
	if result.Error == nil || result.Error.Code != ErrorCodeMissingRequest {
		t.Fatalf("error = %v, want missing_request", result.Error)
	}
}

func TestCallToolReturnsUnknownToolError(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{Tool: "does.not.exist"})
	if result.Error == nil || result.Error.Code != ErrorCodeUnknownTool {
		t.Fatalf("error = %v, want unknown_tool", result.Error)
	}
}

func TestCallToolReturnsInvalidArgumentForMalformedQueryJSON(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{QueryRunner: &alwaysFailRunner{}}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "query.execute",
		Args: json.RawMessage(`{"sql":`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeInvalidArgument {
		t.Fatalf("error = %v, want invalid_argument", result.Error)
	}
}

func TestCallToolReturnsInvalidArgumentWhenSqlMissing(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{QueryRunner: &alwaysFailRunner{}}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "query.execute",
		Args: json.RawMessage(`{"sql":"   "}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeInvalidArgument {
		t.Fatalf("error = %v, want invalid_argument", result.Error)
	}
}

func TestCallToolReturnsHandlerErrorWhenQueryRunnerFails(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{QueryRunner: &alwaysFailRunner{}}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "query.execute",
		Args: json.RawMessage(`{"sql":"SELECT 1"}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeHandlerError {
		t.Fatalf("error = %v, want handler_error", result.Error)
	}
	if !strings.Contains(result.Error.Message, "query engine unavailable") {
		t.Fatalf("error message = %q", result.Error.Message)
	}
}

func TestCallToolReturnsUnauthorizedWhenPrincipalNotAllowed(t *testing.T) {
	server := NewServer(nil, nil)
	if err := RegisterCoreTools(server, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool:      "system.ping",
		Principal: "blocked",
	})
	if result.Error == nil || result.Error.Code != ErrorCodeUnauthorized {
		t.Fatalf("error = %v, want unauthorized", result.Error)
	}
}
