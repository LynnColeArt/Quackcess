package query

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

const largeQueryBudget = 3 * time.Second

func TestLargeProjectQueryMeetsLatencyBudget(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "query.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	if err := seedLargeQueryDataset(database.SQL); err != nil {
		t.Fatalf("seed dataset: %v", err)
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
			{Source: "c", Column: "id"},
			{Source: "c", Column: "region"},
		},
		GroupBy: []FieldRef{
			{Source: "c", Column: "id"},
			{Source: "c", Column: "region"},
		},
		OrderBy: []OrderBy{
			{Source: "c", Column: "id", Desc: false},
		},
	}

	// warm-up for allocator/JIT/cache effects
	if _, err := ExecuteGraph(database.SQL, graph); err != nil {
		t.Fatalf("warm-up execute graph: %v", err)
	}

	start := time.Now()
	result, err := ExecuteGraph(database.SQL, graph)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("execute graph: %v", err)
	}
	if elapsed > largeQueryBudget {
		t.Fatalf("large query exceeded %s budget: %s", largeQueryBudget, elapsed)
	}
	if result.RowCount == 0 {
		t.Fatalf("expected at least one row in query result")
	}
}

func seedLargeQueryDataset(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}

	if _, err := database.Exec(`CREATE TABLE customers (id BIGINT, region INTEGER, lifetime BIGINT);`); err != nil {
		return err
	}
	if _, err := database.Exec(`INSERT INTO customers SELECT i, i % 128, i FROM range(0, 50000) AS t(i);`); err != nil {
		return err
	}

	if _, err := database.Exec(`CREATE TABLE orders (id BIGINT, customer_id BIGINT, amount BIGINT);`); err != nil {
		return err
	}
	if _, err := database.Exec(`INSERT INTO orders SELECT i, i % 50000, i % 400 FROM range(0, 300000) AS t(i);`); err != nil {
		return err
	}
	return nil
}
