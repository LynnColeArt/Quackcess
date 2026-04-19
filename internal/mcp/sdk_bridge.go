package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// SDKServerOptions mirrors the subset of [sdk.ServerOptions] we currently
// need to pass through when constructing a public MCP server.
type SDKServerOptions struct {
	Implementation *sdk.Implementation
	Options        *sdk.ServerOptions
}

// NewSDKServer creates a Go SDK MCP server backed by the internal mcp tool core.
//
// The returned server is fully wired with core tools and can be passed to
// [sdk.Server.Run].
func NewSDKServer(core *Server, options SDKServerOptions, defaultPrincipal string) (*sdk.Server, error) {
	if core == nil {
		return nil, fmt.Errorf("core server is nil")
	}
	if options.Implementation == nil {
		return nil, fmt.Errorf("implementation is required")
	}

	server := sdk.NewServer(options.Implementation, options.Options)
	tools := core.ListTools()
	for _, tool := range tools {
		definition := tool
		sdk.AddTool(
			server,
			&sdk.Tool{
				Name:        definition.Name,
				Description: definition.Description,
				InputSchema: map[string]any{"type": "object"},
			},
			func(ctx context.Context, request *sdk.CallToolRequest, args any) (*sdk.CallToolResult, any, error) {
				return handleSDKCall(ctx, core, definition.Name, request, args, defaultPrincipal)
			},
		)
	}

	return server, nil
}

func handleSDKCall(
	ctx context.Context,
	core *Server,
	toolName string,
	request *sdk.CallToolRequest,
	args any,
	defaultPrincipal string,
) (*sdk.CallToolResult, any, error) {
	_ = ctx

	callArgs := marshalSDKArguments(args)
	result := core.CallTool(context.Background(), &CallRequest{
		Tool:      toolName,
		Principal: resolvePrincipal(request, defaultPrincipal),
		RequestID: resolveRequestID(request),
		Args:      callArgs,
	})

	if result.Error != nil {
		return &sdk.CallToolResult{
			IsError: true,
			StructuredContent: map[string]any{
				"tool":  result.Tool,
				"error": toolErrorPayload(result.Error),
			},
			Content: []sdk.Content{
				&sdk.TextContent{
					Text: result.Error.Error(),
				},
			},
		}, nil, nil
	}

	return &sdk.CallToolResult{
		StructuredContent: map[string]any{
			"tool": result.Tool,
			"data": result.Data,
		},
	}, nil, nil
}

func toolErrorPayload(err *ToolError) map[string]any {
	if err == nil {
		return nil
	}
	return map[string]any{
		"code":    err.Code,
		"message": err.Message,
	}
}

func marshalSDKArguments(args any) json.RawMessage {
	if args == nil {
		return []byte("{}")
	}
	if bytes, ok := args.(json.RawMessage); ok {
		if len(bytes) == 0 {
			return []byte("{}")
		}
		return bytes
	}

	data, err := json.Marshal(args)
	if err != nil {
		return []byte("{}")
	}
	if len(data) == 0 {
		return []byte("{}")
	}
	return data
}

func resolvePrincipal(request *sdk.CallToolRequest, fallback string) string {
	if request == nil || request.Params == nil || len(request.Params.Meta) == 0 {
		return fallback
	}
	if principal, ok := request.Params.Meta["principal"].(string); ok && strings.TrimSpace(principal) != "" {
		return principal
	}
	return fallback
}

func resolveRequestID(request *sdk.CallToolRequest) string {
	if request == nil || request.Params == nil || len(request.Params.Meta) == 0 {
		return ""
	}
	if requestID, ok := request.Params.Meta["requestId"].(string); ok {
		return requestID
	}
	return ""
}
