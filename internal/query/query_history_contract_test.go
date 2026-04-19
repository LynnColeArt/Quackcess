package query

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestQueryHistoryRepositoryLogsAndLists(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "history.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repository := NewQueryHistoryRepository(database.SQL)

	if err := repository.Log("SELECT 1", []any{}, 1, 5, nil); err != nil {
		t.Fatalf("log success: %v", err)
	}
	if err := repository.Log("SELECT 2 WHERE x = ?", []any{"x"}, 0, 7, errors.New("bad sql")); err != nil {
		t.Fatalf("log failure: %v", err)
	}

	entries, err := repository.ListRecent(10)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	var latestFailure *QueryHistory
	var latestSuccess *QueryHistory
	for _, entry := range entries {
		switch entry.SQLText {
		case "SELECT 2 WHERE x = ?":
			latestFailure = &entry
		case "SELECT 1":
			latestSuccess = &entry
		}
	}

	if latestFailure == nil {
		t.Fatalf("failed entry not found: %#v", entries)
	}
	if latestSuccess == nil {
		t.Fatalf("success entry not found: %#v", entries)
	}
	if latestFailure.Success {
		t.Fatalf("expected failed entry for SELECT 2 query, got %#v", latestFailure)
	}
	if !latestSuccess.Success {
		t.Fatalf("expected successful entry for SELECT 1 query, got %#v", latestSuccess)
	}
	if latestSuccess.RowCount != 1 {
		t.Fatalf("success row_count = %d, want 1", latestSuccess.RowCount)
	}
	if latestFailure.ErrorText == "" {
		t.Fatalf("expected error text for failed entry")
	}

	parsed := []any{}
	if err := json.Unmarshal([]byte(latestFailure.ParametersJSON), &parsed); err != nil {
		t.Fatalf("parameters should be parseable: %v", err)
	}
	if len(parsed) != 1 || parsed[0] != "x" {
		t.Fatalf("unexpected parsed params: %#v", parsed)
	}
}
