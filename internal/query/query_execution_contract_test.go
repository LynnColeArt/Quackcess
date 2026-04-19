package query

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestQueryExecutionReturnsRows(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "query.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	if _, err := seed.Exec(`CREATE TABLE customers(id BIGINT, name TEXT);`); err != nil {
		t.Fatalf("create customers: %v", err)
	}
	if _, err := seed.Exec(`CREATE TABLE orders(id BIGINT, customer_id BIGINT, total BIGINT);`); err != nil {
		t.Fatalf("create orders: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO customers VALUES (1, 'Alice'), (2, 'Bob');`); err != nil {
		t.Fatalf("seed customers: %v", err)
	}
	if _, err := seed.Exec(`INSERT INTO orders VALUES (10, 1, 100), (11, 1, 50), (12, 2, 0);`); err != nil {
		t.Fatalf("seed orders: %v", err)
	}

	graph := QueryGraph{
		From: QuerySource{Table: "customers", Alias: "c"},
		Joins: []Join{
			{
				Type:        JoinLeft,
				LeftAlias:   "c",
				LeftColumn:  "id",
				RightTable:  "orders",
				RightAlias:  "o",
				RightColumn: "customer_id",
			},
		},
		Fields: []FieldRef{
			{Source: "c", Column: "name"},
		},
		Where:   `o.total > 0`,
		OrderBy: []OrderBy{{Source: "c", Column: "name"}},
	}

	result, err := ExecuteGraph(seed, graph)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if result.RowCount != 2 {
		t.Fatalf("rowCount = %d, want 2", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "name" {
		t.Fatalf("unexpected columns: %#v", result.Columns)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(result.Rows))
	}

	if got, want := result.Rows[0][0], "Alice"; got != want {
		t.Fatalf("row0[0] = %#v, want %q", got, want)
	}

	repo := NewQueryHistoryRepository(seed)
	entries, err := repo.ListRecent(10)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(history entries) = %d, want 1", len(entries))
	}

	entry := entries[0]
	if !entry.Success {
		t.Fatalf("history entry should be successful")
	}
	if entry.RowCount != result.RowCount {
		t.Fatalf("history row_count = %d, want %d", entry.RowCount, result.RowCount)
	}
	if !strings.Contains(entry.SQLText, result.SQL) {
		t.Fatalf("history SQL does not include executed SQL: %#v", entry.SQLText)
	}
}

func TestExecuteSQLFailureLogsHistory(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "query-failure.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	seed := database.SQL
	_, err = ExecuteSQL(seed, "SELECT * FROM missing_table")
	if err == nil {
		t.Fatal("expected execute failure")
	}

	repo := NewQueryHistoryRepository(seed)
	entries, err := repo.ListRecent(10)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(history entries) = %d, want 1", len(entries))
	}
	if entries[0].Success {
		t.Fatal("history entry should be marked as failure")
	}
	if entries[0].ErrorText == "" {
		t.Fatal("history entry should include error text")
	}
}
