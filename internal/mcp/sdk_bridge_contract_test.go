package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	qsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewSDKServerExposesToolsOverMCPTransport(t *testing.T) {
	core, _, _, _ := newContractServer(t)
	mcpServer, err := NewSDKServer(core, SDKServerOptions{
		Implementation: &qsdk.Implementation{
			Name:    "quackcess-test",
			Version: "0.0.0",
		},
	}, "agent")
	if err != nil {
		t.Fatalf("NewSDKServer: %v", err)
	}

	client := connectSDKServerForTest(t, mcpServer)
	result, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 7 {
		t.Fatalf("tools = %d, want 7", len(result.Tools))
	}

	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	if !reflect.DeepEqual(names, []string{
		"artifact.delete",
		"artifact.get",
		"artifact.list",
		"artifact.set",
		"query.execute",
		"schema.inspect",
		"system.ping",
	}) {
		t.Fatalf("tool names = %#v", names)
	}
}

func TestSDKServerCallExecutesCoreToolWithPrincipal(t *testing.T) {
	core, _, _, _ := newContractServer(t)
	mcpServer, err := NewSDKServer(core, SDKServerOptions{
		Implementation: &qsdk.Implementation{
			Name:    "quackcess-test",
			Version: "0.0.0",
		},
	}, "agent")
	if err != nil {
		t.Fatalf("NewSDKServer: %v", err)
	}

	client := connectSDKServerForTest(t, mcpServer)
	got, err := client.CallTool(context.Background(), &qsdk.CallToolParams{
		Name: "system.ping",
		Arguments: map[string]any{
			"ignored": "value",
		},
		Meta: qsdk.Meta{"principal": "alice"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	payload, ok := got.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T", got.StructuredContent)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("payload data type = %T", payload["data"])
	}
	if data["principal"] != "alice" {
		t.Fatalf("principal in ping data = %#v", data["principal"])
	}
	if data["pong"] != true {
		t.Fatalf("pong = %#v", data["pong"])
	}
}

func TestSDKServerConvertsToolErrorsIntoMCPToolErrors(t *testing.T) {
	core := NewServer(NewAllowlistAuthorizer(false), nil)
	if err := RegisterCoreTools(core, CoreTools{}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	server, err := NewSDKServer(core, SDKServerOptions{
		Implementation: &qsdk.Implementation{
			Name:    "quackcess-test",
			Version: "0.0.0",
		},
	}, "agent")
	if err != nil {
		t.Fatalf("NewSDKServer: %v", err)
	}

	client := connectSDKServerForTest(t, server)
	got, err := client.CallTool(context.Background(), &qsdk.CallToolParams{
		Name: "system.ping",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !got.IsError {
		t.Fatal("expected tool error result")
	}

	payload, ok := got.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T", got.StructuredContent)
	}
	errPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("error payload type = %T", payload["error"])
	}
	if errPayload["code"] != ErrorCodeUnauthorized {
		t.Fatalf("error code = %v, want %s", errPayload["code"], ErrorCodeUnauthorized)
	}
}

func TestMarshalSDKArgumentsAcceptsAnyInputs(t *testing.T) {
	raw := marshalSDKArguments(map[string]any{"sql": "SELECT 1", "n": 2})
	if raw == nil {
		t.Fatal("expected marshaled args")
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if decoded["sql"] != "SELECT 1" {
		t.Fatalf("sql = %#v", decoded["sql"])
	}
	if got := decoded["n"]; got != float64(2) {
		t.Fatalf("n = %#v, want float64(2)", got)
	}
}

func connectSDKServerForTest(t *testing.T, sdkServer *qsdk.Server) *qsdk.ClientSession {
	t.Helper()
	serverTransport, clientTransport := qsdk.NewInMemoryTransports()

	serverSession, err := sdkServer.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := qsdk.NewClient(&qsdk.Implementation{
		Name:    "quackcess-client-test",
		Version: "0.0.0",
	}, nil)

	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		serverSession.Close()
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() {
		clientSession.Close()
		serverSession.Close()
	})

	return clientSession
}
