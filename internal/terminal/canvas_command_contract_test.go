package terminal

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestTerminalCanvasNewRequiresCanvasService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-new-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)
	result, err := service.RunCommand(`\canvas new missing-canvas`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "canvas service is not configured" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "canvas service is not configured")
	}
}

func TestTerminalCanvasRenameRequiresCanvasService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-rename-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)
	result, err := service.RunCommand(`\canvas rename old-name new-name`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "canvas service is not configured" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "canvas service is not configured")
	}
}

func TestTerminalCanvasDeleteRequiresCanvasService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-delete-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)
	result, err := service.RunCommand(`\canvas delete missing-canvas`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "canvas service is not configured" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "canvas service is not configured")
	}
}

func TestTerminalCanvasSaveRequiresCanvasService(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-save-service.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)
	result, err := service.RunCommand(`\canvas save missing '{"nodes":[]}'`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText != "canvas service is not configured" {
		t.Fatalf("error = %q, want %q", result.ErrorText, "canvas service is not configured")
	}
}

func TestTerminalCanvasMalformedNewCommandReturnsUsageError(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-new-malformed.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, nil)
	result, err := service.RunCommand(`\canvas new`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "canvas usage: \\canvas new <name>") {
		t.Fatalf("error = %q, expected usage hint", result.ErrorText)
	}
}

func TestTerminalCanvasMalformedRenameCommandReturnsUsageError(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-rename-malformed.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, nil)
	result, err := service.RunCommand(`\canvas rename legacy`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "canvas usage: \\canvas rename <old-name> <new-name>") {
		t.Fatalf("error = %q, expected usage hint", result.ErrorText)
	}
}

func TestTerminalCanvasMalformedDeleteCommandReturnsUsageError(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-delete-malformed.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, nil)
	result, err := service.RunCommand(`\canvas delete`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "canvas usage: \\canvas delete <name>") {
		t.Fatalf("error = %q, expected usage hint", result.ErrorText)
	}
}

func TestTerminalCanvasMalformedSaveCommandReturnsUsageError(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-save-malformed.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, nil)
	result, err := service.RunCommand(`\canvas save bad`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "canvas usage: \\canvas save <name> <spec-json>") {
		t.Fatalf("error = %q, expected usage hint", result.ErrorText)
	}
}
