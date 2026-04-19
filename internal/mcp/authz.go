package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const wildcardTool = "*"
const wildcardPrincipal = "*"

type Authorizer interface {
	CanAccess(principal string, tool string) bool
}

type AllowlistAuthorizer struct {
	rules        map[string]map[string]struct{}
	defaultAllow bool
}

type PermissionMatrix struct {
	DefaultAllow bool                `json:"defaultAllow"`
	Principals   map[string][]string `json:"principals"`
}

func NewAllowlistAuthorizer(defaultAllow bool) *AllowlistAuthorizer {
	return &AllowlistAuthorizer{
		rules:        map[string]map[string]struct{}{},
		defaultAllow: defaultAllow,
	}
}

func (a *AllowlistAuthorizer) Clone() *AllowlistAuthorizer {
	if a == nil {
		return nil
	}
	return &AllowlistAuthorizer{
		rules:        cloneToolMap(a.rules),
		defaultAllow: a.defaultAllow,
	}
}

func cloneToolMap(source map[string]map[string]struct{}) map[string]map[string]struct{} {
	target := map[string]map[string]struct{}{}
	for principal, tools := range source {
		if tools == nil {
			target[principal] = nil
			continue
		}
		next := map[string]struct{}{}
		for tool := range tools {
			next[tool] = struct{}{}
		}
		target[principal] = next
	}
	return target
}

func (a *AllowlistAuthorizer) Grant(principal, tool string) {
	if a == nil {
		return
	}
	if a.rules[principal] == nil {
		a.rules[principal] = map[string]struct{}{}
	}
	a.rules[principal][tool] = struct{}{}
}

func (a *AllowlistAuthorizer) CanAccess(principal string, tool string) bool {
	if a == nil {
		return false
	}
	if a.defaultAllow {
		return true
	}
	principalRules, ok := a.rules[strings.TrimSpace(principal)]
	if ok && principalRules != nil {
		if _, has := principalRules[wildcardTool]; has {
			return true
		}
		if _, has := principalRules[tool]; has {
			return true
		}
	}

	if wildcardPrincipalRules, has := a.rules[wildcardPrincipal]; has && wildcardPrincipalRules != nil {
		if _, has := wildcardPrincipalRules[wildcardTool]; has {
			return true
		}
		if _, has := wildcardPrincipalRules[tool]; has {
			return true
		}
	}
	return false
}

func ParsePermissionMatrix(raw []byte) (*AllowlistAuthorizer, error) {
	var matrix PermissionMatrix
	if len(raw) == 0 {
		return nil, fmt.Errorf("permission matrix payload is empty")
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		return nil, fmt.Errorf("decode permission matrix: %w", err)
	}

	authorizer := NewAllowlistAuthorizer(matrix.DefaultAllow)
	for principal, tools := range matrix.Principals {
		normalizedPrincipal := strings.TrimSpace(principal)
		if normalizedPrincipal == "" {
			continue
		}
		for _, tool := range tools {
			if normalizedTool := strings.TrimSpace(tool); normalizedTool != "" {
				authorizer.Grant(normalizedPrincipal, normalizedTool)
			}
		}
	}
	return authorizer, nil
}

func LoadPermissionMatrix(path string) (*AllowlistAuthorizer, error) {
	rawPath := strings.TrimSpace(path)
	if rawPath == "" {
		return nil, fmt.Errorf("permission matrix path is required")
	}
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, fmt.Errorf("read permission matrix: %w", err)
	}
	return ParsePermissionMatrix(raw)
}
