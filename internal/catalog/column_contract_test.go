package catalog

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCatalogColumnCrudFlow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	tableRepo := NewTableRepository(database.SQL)
	if err := tableRepo.Create("projects", "CREATE TABLE projects(id BIGINT PRIMARY KEY, name TEXT);"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	repo := NewColumnRepository(database.SQL)
	if err := repo.Create(Column{
		TableName:    "projects",
		Name:         "id",
		Position:     1,
		DataType:     "BIGINT",
		IsPrimaryKey: true,
		IsNullable:   false,
	}); err != nil {
		t.Fatalf("create id column: %v", err)
	}
	if err := repo.Create(Column{
		TableName:  "projects",
		Name:       "name",
		Position:   2,
		DataType:   "TEXT",
		IsNullable: true,
	}); err != nil {
		t.Fatalf("create name column: %v", err)
	}

	columns, err := repo.List("projects")
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}
	if len(columns) != 2 {
		t.Fatalf("len(columns) = %d, want 2", len(columns))
	}
	if columns[0].Name != "id" || columns[1].Name != "name" {
		t.Fatalf("unexpected order: %#v", columns)
	}

	if err := repo.Delete("projects", "name"); err != nil {
		t.Fatalf("delete column: %v", err)
	}

	columns, err = repo.List("projects")
	if err != nil {
		t.Fatalf("list columns after delete: %v", err)
	}
	if len(columns) != 1 || columns[0].Name != "id" {
		t.Fatalf("unexpected columns after delete: %#v", columns)
	}
}

func TestCatalogColumnRepositoryRejectsBadDefinition(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewColumnRepository(database.SQL)
	if err := repo.Create(Column{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCatalogColumnRepositoryRejectsDuplicatePosition(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	tableRepo := NewTableRepository(database.SQL)
	if err := tableRepo.Create("events", "CREATE TABLE events(id BIGINT, created_at TIMESTAMP);"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	repo := NewColumnRepository(database.SQL)
	if err := repo.Create(Column{TableName: "events", Name: "id", Position: 1, DataType: "BIGINT"}); err != nil {
		t.Fatalf("create first column: %v", err)
	}
	err = repo.Create(Column{TableName: "events", Name: "created_at", Position: 1, DataType: "TIMESTAMP"})
	if err == nil {
		t.Fatal("expected duplicate position error")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
		!strings.Contains(err.Error(), "already exists") &&
		!strings.Contains(err.Error(), "Duplicate key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
