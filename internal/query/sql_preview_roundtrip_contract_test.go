package query

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestPreviewSQLIsExecutable(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "roundtrip.duckdb")

	database, err := db.Bootstrap(dbPath)
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

	graph := QueryGraph{
		From: QuerySource{Table: "products", Alias: "p"},
		Fields: []FieldRef{
			{Source: "p", Column: "sku"},
		},
		OrderBy: []OrderBy{{Expression: `"p"."sku"`}},
	}

	sqlText, err := GenerateSQL(graph)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(sqlText.SQL, `FROM "products" "p"`) {
		t.Fatalf("unexpected preview SQL: %s", sqlText.SQL)
	}

	result, err := ExecuteSQL(database.SQL, sqlText.SQL)
	if err != nil {
		t.Fatalf("execute sql: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("rowCount = %d, want 2", result.RowCount)
	}
}
