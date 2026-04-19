package terminal

import (
	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/query"

	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalExecutesSelectAsQuery(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	if _, err := seed.Exec(`CREATE TABLE products(id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create products: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO products VALUES (1, 'ABC'), (2, 'DEF');`); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	service := NewTerminalService(seed)

	result, err := service.RunCommand("SELECT sku FROM products ORDER BY id")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "sku" {
		t.Fatalf("unexpected columns: %#v", result.Columns)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(result.Rows))
	}
	if len(result.Rows) > 0 {
		if got, want := result.Rows[0][0], "ABC"; got != want {
			t.Fatalf("row0[0] = %#v, want %q", got, want)
		}
	}
}

func TestTerminalHelpCommandIncludesVectorizeUsage(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-help.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)

	result, err := service.RunCommand("\\help")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if !strings.Contains(result.Message, "UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>]") {
		t.Fatalf("help message missing vectorize usage: %q", result.Message)
	}
}

func TestTerminalReturnsHistoryCommand(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	if _, err := seed.Exec(`CREATE TABLE products(id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create products: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO products VALUES (1, 'ABC');`); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	service := NewTerminalService(seed)
	if _, err := service.RunCommand("SELECT sku FROM products"); err != nil {
		t.Fatalf("run query command: %v", err)
	}
	history, err := service.RunCommand("\\history")
	if err != nil {
		t.Fatalf("run history command: %v", err)
	}
	if history.Kind != TerminalKindHistory {
		t.Fatalf("kind = %q, want %q", history.Kind, TerminalKindHistory)
	}
	if history.RowCount < 1 {
		t.Fatalf("row count = %d, want >= 1", history.RowCount)
	}
	if history.Message == "" {
		t.Fatalf("expected history message")
	}
	if !strings.Contains(history.Message, "SELECT sku FROM products") {
		t.Fatalf("history message missing sql: %q", history.Message)
	}
}

func TestTerminalInvalidCommandReturnsErrorResult(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-error.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)

	result, err := service.RunCommand("SELECT * FROM not_a_table")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText == "" {
		t.Fatalf("expected error text")
	}
}

func TestTerminalExecutesCanvasByName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-canvas.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	if _, err := seed.Exec(`CREATE TABLE products(id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create products: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO products VALUES (1, 'ABC'), (2, 'DEF');`); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	canvasRepo := catalog.NewCanvasRepository(seed)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "canvas-sales",
		Name:     "sales",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"products","alias":"p","fields":[{"name":"sku","alias":"sku_code"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	service := NewTerminalServiceWithCanvasRepository(seed, nil, canvasRepo)
	result, err := service.RunCommand("\\canvas sales")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if !strings.Contains(result.SQLText, `SELECT "p"."sku" AS "sku_code" FROM "products" "p"`) {
		t.Fatalf("sql = %q, want contains %q", result.SQLText, `SELECT "p"."sku" AS "sku_code" FROM "products" "p"`)
	}
	if !strings.Contains(result.SQLText, "LIMIT") {
		t.Fatalf("sql = %q, expected LIMIT for safety", result.SQLText)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(result.Rows))
	}
}

func TestTerminalCanvasExecutionUsesSafeRowLimit(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-canvas-limit.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	if _, err := seed.Exec(`CREATE TABLE products(id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create products: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO products VALUES (1, 'ABC'), (2, 'DEF'), (3, 'GHI');`); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	canvasRepo := catalog.NewCanvasRepository(seed)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "canvas-sales",
		Name:     "sales",
		Kind:     "query",
		SpecJSON: `{"nodes":[{"id":"n1","kind":"table","table":"products","alias":"p","fields":[{"name":"sku","alias":"sku_code"}]}],"edges":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}

	service := NewTerminalServiceWithCanvasRepository(seed, nil, canvasRepo)
	result, err := service.RunCommand("\\canvas sales")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindQuery {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindQuery)
	}
	if !strings.Contains(result.SQLText, "LIMIT 200") {
		t.Fatalf("sql = %q, expected LIMIT 200", result.SQLText)
	}
}

func TestTerminalCanvasCommandFailsWhenCanvasMissing(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-canvas-missing.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	result, err := service.RunCommand("\\canvas unknown")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
	if result.ErrorText == "" {
		t.Fatalf("expected error text")
	}
}

func TestTerminalCanvasRequiresRepository(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-canvas-no-repo.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	service := NewTerminalService(database.SQL)

	result, err := service.RunCommand("\\canvas anything")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindError {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindError)
	}
}

func TestTerminalCanvasNewCommandCreatesCanvas(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-new.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	result, err := service.RunCommand("\\canvas new sales-overview")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if result.Message != "created canvas sales-overview" {
		t.Fatalf("message = %q, want %q", result.Message, "created canvas sales-overview")
	}

	if _, err := canvasRepo.FindByName("sales-overview"); err != nil {
		t.Fatalf("find created: %v", err)
	}
}

func TestTerminalCanvasRenameCommand(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-rename.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "canvas-rename",
		Name:     "sales-overview",
		Kind:     "query",
		SpecJSON: `{"nodes":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	result, err := service.RunCommand("\\canvas rename sales-overview sales-yearly")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if result.Message != "renamed canvas sales-overview -> sales-yearly" {
		t.Fatalf("message = %q, want %q", result.Message, "renamed canvas sales-overview -> sales-yearly")
	}
	_, err = canvasRepo.FindByName("sales-overview")
	if err == nil {
		t.Fatal("expected old name to be gone")
	}
	if _, err := canvasRepo.FindByName("sales-yearly"); err != nil {
		t.Fatalf("find renamed canvas: %v", err)
	}
}

func TestTerminalCanvasDeleteCommand(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-delete.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "delete-id",
		Name:     "to-delete",
		Kind:     "query",
		SpecJSON: `{"nodes":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	result, err := service.RunCommand("\\canvas delete to-delete")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	if _, err := canvasRepo.ListByKind("query"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, err := canvasRepo.FindByName("to-delete"); err == nil {
		t.Fatalf("expected deleted canvas")
	}
}

func TestTerminalCanvasSaveCommand(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Bootstrap(filepath.Join(tmp, "terminal-canvas-save.duckdb"))
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	canvasRepo := catalog.NewCanvasRepository(database.SQL)
	if err := canvasRepo.Create(catalog.Canvas{
		ID:       "save-id",
		Name:     "to-save",
		Kind:     "query",
		SpecJSON: `{"nodes":[]}`,
	}); err != nil {
		t.Fatalf("create canvas: %v", err)
	}
	service := NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepo)

	result, err := service.RunCommand(`\canvas save to-save '{"nodes":[{"id":"n1","kind":"table","table":"customers","fields":[{"name":"id"}]}],"edges":[]}'`)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.Kind != TerminalKindHelp {
		t.Fatalf("kind = %q, want %q", result.Kind, TerminalKindHelp)
	}
	saved, err := canvasRepo.FindByName("to-save")
	if err != nil {
		t.Fatalf("find saved: %v", err)
	}
	savedSpec, err := query.ParseCanvasSpec([]byte(saved.SpecJSON))
	if err != nil {
		t.Fatalf("parse saved spec: %v", err)
	}
	if len(savedSpec.Nodes) != 1 || savedSpec.Nodes[0].ID != "n1" {
		t.Fatalf("saved spec = %#v", savedSpec)
	}
}

func TestTerminalPublishesEventsToConsole(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "terminal-events.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	if _, err := seed.Exec(`CREATE TABLE products(id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create products: %v", err)
	}

	console := NewEventConsole(10)
	service := NewTerminalServiceWithConsole(seed, console)

	_, err = service.RunCommand("CREATE TABLE noargs(")
	if err != nil {
		t.Fatalf("run invalid command: %v", err)
	}

	if _, err := service.RunCommand("CREATE TABLE products2(id BIGINT);"); err != nil {
		t.Fatalf("run create command: %v", err)
	}

	if _, err := service.RunCommand("SELECT * FROM products2"); err != nil {
		t.Fatalf("run select command: %v", err)
	}

	events := console.Items()
	if len(events) < 3 {
		t.Fatalf("len(events) = %d, want >= 3", len(events))
	}
	last := events[len(events)-1]
	if last.Kind != "query.executed" {
		t.Fatalf("last event kind = %q, want query.executed", last.Kind)
	}

	failedFound := false
	executedFound := false
	for _, event := range events {
		switch event.Kind {
		case "query.failed":
			failedFound = true
		case "query.executed":
			executedFound = true
		}
	}
	if !failedFound || !executedFound {
		t.Fatalf("expected both failed and executed events, got %#v", events)
	}
}
