package project

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/canvasservice"
	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/db"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

func TestProjectRoundtripPreservesCanvasCreateEditRunAndReopen(t *testing.T) {
	tmp := t.TempDir()
	projectDBPath := filepath.Join(tmp, "workspace.duckdb")
	database, err := db.Bootstrap(projectDBPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if _, err := database.SQL.Exec(`CREATE TABLE customers (id BIGINT, name TEXT);`); err != nil {
		t.Fatalf("create customers: %v", err)
	}
	if _, err := database.SQL.Exec(`CREATE TABLE orders (id BIGINT, customer_id BIGINT, sku TEXT);`); err != nil {
		t.Fatalf("create orders: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO customers VALUES (1, 'Alice'), (2, 'Bob');`); err != nil {
		t.Fatalf("insert customers: %v", err)
	}
	if _, err := database.SQL.Exec(`INSERT INTO orders VALUES (1, 1, 'A1'), (2, 2, 'B1');`); err != nil {
		t.Fatalf("insert orders: %v", err)
	}

	canvasRepository := catalog.NewCanvasRepository(database.SQL)
	canvasService := canvasservice.NewCanvasArtifactService(canvasRepository)
	terminalService := terminal.NewTerminalServiceWithCanvasRepository(database.SQL, nil, canvasRepository)

	if _, err := canvasService.CreateDraftCanvas("sales"); err != nil {
		t.Fatalf("create draft canvas: %v", err)
	}

	if err := canvasService.SaveCanvasSpec("sales",
		`{"nodes":[
			{"id":"customers","kind":"table","table":"customers","alias":"c","fields":[{"name":"id"},{"name":"name"}]},
			{"id":"orders","kind":"table","table":"orders","alias":"o","fields":[{"name":"id"},{"name":"customer_id"},{"name":"sku"}]}
		],"edges":[
			{"id":"join-1","kind":"join","from":"customers","to":"orders","fromColumn":"id","toColumn":"customer_id","joinType":"LEFT"}
		]}`,
		"ui",
	); err != nil {
		t.Fatalf("save canvas spec: %v", err)
	}

	result, err := terminalService.RunCommand(`\canvas sales`)
	if err != nil {
		t.Fatalf("run canvas: %v", err)
	}
	if result.Kind != terminal.TerminalKindQuery {
		t.Fatalf("result kind = %q, want %q", result.Kind, terminal.TerminalKindQuery)
	}
	if result.RowCount != 2 {
		t.Fatalf("row count = %d, want 2", result.RowCount)
	}

	canvas, err := canvasRepository.FindByName("sales")
	if err != nil {
		t.Fatalf("find saved canvas: %v", err)
	}
	if canvas.Version != 2 {
		t.Fatalf("canvas version = %d, want 2", canvas.Version)
	}
	spec, err := query.ParseCanvasSpec([]byte(canvas.SpecJSON))
	if err != nil {
		t.Fatalf("parse saved spec: %v", err)
	}
	if len(spec.Edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(spec.Edges))
	}

	if err := database.Close(); err != nil {
		t.Fatalf("close source db: %v", err)
	}

	manifest := DefaultManifest()
	manifest.ProjectName = "Sales"
	manifest.CreatedBy = "tester"

	projectPath := filepath.Join(tmp, "sales-project.qdb")
	if err := Create(projectPath, CreateOptions{Manifest: manifest, DatabaseSourcePath: projectDBPath}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	reopenedPath := filepath.Join(tmp, "reopened.duckdb")
	reopenedPayload, err := project.ReadDataFile()
	if err != nil {
		t.Fatalf("read data file: %v", err)
	}
	if len(reopenedPayload) == 0 {
		t.Fatal("expected non-empty data payload")
	}
	if err := os.WriteFile(reopenedPath, reopenedPayload, 0o644); err != nil {
		t.Fatalf("write reopened db bytes: %v", err)
	}

	reopened, err := db.Bootstrap(reopenedPath)
	if err != nil {
		t.Fatalf("bootstrap reopened: %v", err)
	}
	defer reopened.Close()

	reopenedRepo := catalog.NewCanvasRepository(reopened.SQL)
	reopenedCanvas, err := reopenedRepo.FindByName("sales")
	if err != nil {
		t.Fatalf("find reopened canvas: %v", err)
	}
	if reopenedCanvas.ID != canvas.ID {
		t.Fatalf("canvas id = %q, want %q", reopenedCanvas.ID, canvas.ID)
	}
	if reopenedCanvas.Version != canvas.Version {
		t.Fatalf("canvas version = %d, want %d", reopenedCanvas.Version, canvas.Version)
	}

	reopenedTerminal := terminal.NewTerminalServiceWithCanvasRepository(reopened.SQL, nil, reopenedRepo)
	reopenResult, err := reopenedTerminal.RunCommand(`\canvas sales`)
	if err != nil {
		t.Fatalf("run reopened canvas: %v", err)
	}
	if reopenResult.Kind != terminal.TerminalKindQuery {
		t.Fatalf("reopen result kind = %q, want %q", reopenResult.Kind, terminal.TerminalKindQuery)
	}
	if reopenResult.RowCount != result.RowCount {
		t.Fatalf("reopened row count = %d, want %d", reopenResult.RowCount, result.RowCount)
	}
	if !bytes.Contains([]byte(reopenResult.SQLText), []byte("LEFT JOIN")) {
		t.Fatalf("reopened sql text = %q, expected LEFT JOIN", reopenResult.SQLText)
	}
}
