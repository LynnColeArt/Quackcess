package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ErrorCodeUnknownTool     = "unknown_tool"
	ErrorCodeUnauthorized    = "unauthorized"
	ErrorCodeInvalidArgument = "invalid_argument"
	ErrorCodeHandlerError    = "handler_error"
	ErrorCodeMissingRequest  = "missing_request"
)

type Server struct {
	mu     sync.RWMutex
	tools  map[string]ToolDefinition
	authz  Authorizer
	events *EventBus
}

func NewServer(authorizer Authorizer, eventBus *EventBus) *Server {
	if eventBus == nil {
		eventBus = NewEventBus()
	}
	if authorizer == nil {
		authorizer = NewAllowlistAuthorizer(false)
	}

	return &Server{
		tools:  map[string]ToolDefinition{},
		authz:  authorizer,
		events: eventBus,
	}
}

func (s *Server) RegisterTool(tool ToolDefinition) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	if strings.TrimSpace(tool.Name) == "" {
		return fmt.Errorf("tool name is required")
	}
	if tool.Handler == nil {
		return fmt.Errorf("tool handler is required")
	}

	name := strings.TrimSpace(tool.Name)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}

	cloned := tool
	cloned.Name = name
	s.tools[name] = cloned
	return nil
}

func (s *Server) ListToolNames() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *Server) ListTools() []ToolDefinition {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	s.mu.RUnlock()

	sort.Strings(names)
	tools := make([]ToolDefinition, 0, len(names))
	for _, name := range names {
		tools = append(tools, s.tools[name])
	}
	return tools
}

func (s *Server) CallTool(ctx context.Context, request *CallRequest) (result ToolResult) {
	_ = ctx
	if s == nil {
		return ToolResult{Error: NewToolError(ErrorCodeMissingRequest, "server is nil")}
	}
	if request == nil {
		return ToolResult{Error: NewToolError(ErrorCodeMissingRequest, "request is required")}
	}
	toolName := strings.TrimSpace(request.Tool)
	if toolName == "" {
		return ToolResult{Error: NewToolError(ErrorCodeMissingRequest, "tool name is required")}
	}

	if !s.authz.CanAccess(request.Principal, toolName) {
		s.events.Publish(Event{
			Type:      eventCallDenied,
			Tool:      toolName,
			Principal: request.Principal,
			RequestID: request.RequestID,
			Payload:   map[string]any{"phase": "denied"},
		})
		return ToolResult{
			Tool:  toolName,
			Error: NewToolError(ErrorCodeUnauthorized, "principal is not allowed for this tool"),
		}
	}

	s.mu.RLock()
	tool, exists := s.tools[toolName]
	s.mu.RUnlock()
	if !exists {
		return ToolResult{
			Tool:  toolName,
			Error: NewToolError(ErrorCodeUnknownTool, "tool not found"),
		}
	}

	args := request.Args
	if len(strings.TrimSpace(string(args))) == 0 || strings.TrimSpace(string(args)) == "null" {
		args = json.RawMessage("{}")
	}

	startedAt := time.Now().UTC()
	s.events.Publish(Event{
		Type:      eventCallStarted,
		Tool:      toolName,
		Principal: request.Principal,
		RequestID: request.RequestID,
		Payload:   map[string]any{"phase": "started"},
	})

	result.Tool = toolName
	finishedType := eventCallSuccess
	defer func() {
		if recovered := recover(); recovered != nil {
			finishedType = eventCallFailure
			result = ToolResult{
				Tool:  toolName,
				Error: NewToolError(ErrorCodeHandlerError, fmt.Sprintf("%v", recovered)),
			}
		}

		finishedAt := time.Now().UTC()
		payload := map[string]any{
			"phase":      "finished",
			"durationMs": finishedAt.Sub(startedAt).Milliseconds(),
		}
		if result.Error != nil {
			payload["phase"] = "failure"
			payload["cause"] = result.Error.Error()
		}
		s.events.Publish(Event{
			Type:      finishedType,
			Tool:      toolName,
			Principal: request.Principal,
			RequestID: request.RequestID,
			Payload:   payload,
		})
	}()

	data, handlerErr := tool.Handler(request.Principal, args)
	if handlerErr != nil {
		result.Error = handlerErr
		finishedType = eventCallFailure
		return result
	}
	result.Data = data
	return result
}
