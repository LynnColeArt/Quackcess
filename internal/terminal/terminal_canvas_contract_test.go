package terminal

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/query"
)

func TestTerminalCanvasCommandSequenceHonorsRenameBeforeExecution(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-sequence.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE products(id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create products: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO products VALUES (1, 'ABC'), (2, 'DEF');`); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "seq-1",
		Name:     "sales",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"products","alias":"p","fields":[{"name":"sku"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	result, err := service.RunCommand("\\canvas rename sales sales-export")
	if err != nil {
		t.Fatalf("rename command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if result.Message != "renamed canvas sales -> sales-export" {
		t.Fatalf("message = %q, want %q", result.Message, "renamed canvas sales -> sales-export")
	}
	if _, err := canvasRepo.FindByName("sales"); err == nil {
		t.Fatalf("expected old canvas name to be removed")
	}
	if _, err := canvasRepo.FindByName("sales-export"); err != nil {
		t.Fatalf("expected renamed canvas exists: %v", err)
	}

	result, err = service.RunCommand("\\canvas sales")
	if err != nil {
		t.Fatalf("run old canvas: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "sales") {
		t.Fatalf("error = %q, expected old name in message", result.ErrorText)
	}

	result, err = service.RunCommand("\\canvas sales-export")
	if err != nil {
		t.Fatalf("run renamed canvas: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if !strings.Contains(result.SQLText, `SELECT "p"."sku" FROM "products" "p"`) {
		t.Fatalf("sql = %q, expected products query", result.SQLText)
	}
}

func TestTerminalCanvasSaveCommandRefreshesPersistedSpec(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-save-refresh.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	if _, err := database.SQL.Exec(`CREATE TABLE customers(id BIGINT, status TEXT);`); err != nil {
		t.Fatalf("create customers: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO customers VALUES (1, 'open'), (2, 'closed');`); err != nil {
		t.Fatalf("seed customers: %v", err)
	}

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "save-refresh-id",
		Name:     "dashboard",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	save := `\canvas save dashboard '{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"},{"name":"status"}]}],"edges":[]}'`
	result, err := service.RunCommand(save)
	if err != nil {
		t.Fatalf("save command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if result.Message != "saved canvas dashboard" {
		t.Fatalf("message = %q, want %q", result.Message, "saved canvas dashboard")
	}

	updated, err := canvasRepo.FindByName("dashboard")
	if err != nil {
		t.Fatalf("find canvas after save: %v", err)
	}
	parsed, err := query.ParseCanvasSpec([]byte(updated.SpecJSON))
	if err != nil {
		t.Fatalf("parse saved spec: %v", err)
	}
	if len(parsed.Nodes[0].Fields) != 2 {
		t.Fatalf("saved fields = %d, want 2", len(parsed.Nodes[0].Fields))
	}

	result, err = service.RunCommand("\\canvas dashboard")
	if err != nil {
		t.Fatalf("run saved canvas: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if !strings.Contains(result.SQLText, `"status"`) {
		t.Fatalf("sql = %q, expected status field", result.SQLText)
	}
}

func TestTerminalCanvasDeleteRefreshesRepositoryState(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-delete-refresh.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "delete-refresh-id",
		Name:     "to-remove",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)
	result, err := service.RunCommand("\\canvas delete to-remove")
	if err != nil {
		t.Fatalf("delete command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if result.Message != "deleted canvas to-remove" {
		t.Fatalf("message = %q, want %q", result.Message, "deleted canvas to-remove")
	}
	if _, err := canvasRepo.FindByName("to-remove"); err == nil {
		t.Fatalf("expected to-remove canvas to be deleted")
	}
	result, err = service.RunCommand("\\canvas to-remove")
	if err != nil {
		t.Fatalf("run deleted canvas: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if !strings.Contains(result.ErrorText, "to-remove") {
		t.Fatalf("error = %q, expected canvas name in message", result.ErrorText)
	}
}
