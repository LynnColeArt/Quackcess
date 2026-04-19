package mcp

import (
	"encoding/json"
	"fmt"
)

type ToolError struct {
	Code    string
	Message string
}

func (e *ToolError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewToolError(code, message string) *ToolError {
	return &ToolError{Code: code, Message: message}
}

type ToolResult struct {
	Tool  string      `json:"tool"`
	Data  any         `json:"data,omitempty"`
	Error *ToolError  `json:"error,omitempty"`
}

type ToolHandler func(principal string, args json.RawMessage) (any, *ToolError)

type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema any         `json:"inputSchema,omitempty"`
	Handler     ToolHandler `json:"-"`
}

type CallRequest struct {
	Tool      string          `json:"tool"`
	Principal string          `json:"principal,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
}
