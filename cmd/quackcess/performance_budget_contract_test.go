package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

const openColdStartBudget = 2 * time.Second

func TestOpenCommandColdStartMeetsBudget(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "seed.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close bootstrap db: %v", err)
		}
	}()

	if err := seedLargeOpenDb(database.SQL); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	projectPath := filepath.Join(tmp, "perf-open.qdb")
	if err := run([]string{"init", "--name", "PerfOpen", "--db", dbPath, projectPath}); err != nil {
		t.Fatalf("init: %v", err)
	}

	start := time.Now()
	output, err := captureStdout(func() error {
		return run([]string{"open", "--no-ui", projectPath})
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if strings.TrimSpace(output) != "open mode: headless" {
		t.Fatalf("open output = %q, want %q", strings.TrimSpace(output), "open mode: headless")
	}
	if elapsed > openColdStartBudget {
		t.Fatalf("open command latency %s exceeded %s budget", elapsed, openColdStartBudget)
	}
}

func seedLargeOpenDb(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}
	if _, err := database.Exec(`CREATE TABLE customers AS SELECT i AS id, i % 997 AS region FROM range(0, 50000) AS t(i)`); err != nil {
		return err
	}
	if _, err := database.Exec(`CREATE TABLE events AS SELECT i AS id, i % 50000 AS customer_id, (i * 3) % 1000 AS total FROM range(0, 200000) AS t(i)`); err != nil {
		return err
	}
	return nil
}
