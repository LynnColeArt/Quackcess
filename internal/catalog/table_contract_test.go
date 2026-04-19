package catalog

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCatalogTableCrudFlow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewTableRepository(database.SQL)

	if err := repo.Create("customers", "CREATE TABLE customers(id BIGINT);"); err != nil {
		t.Fatalf("create customers: %v", err)
	}
	if err := repo.Create("projects", "CREATE TABLE projects(id BIGINT);"); err != nil {
		t.Fatalf("create projects: %v", err)
	}

	tables, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tables) != 2 || tables[0].Name != "customers" || tables[1].Name != "projects" {
		t.Fatalf("unexpected tables: %#v", tables)
	}

	if err := repo.Delete("customers"); err != nil {
		t.Fatalf("delete customers: %v", err)
	}

	tables, err = repo.List()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(tables) != 1 || tables[0].Name != "projects" {
		t.Fatalf("unexpected tables after delete: %#v", tables)
	}
}

func TestCatalogTableRepositoryRejectsBadName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewTableRepository(database.SQL)
	if err := repo.Create("", "CREATE TABLE x(id BIGINT);"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCatalogTableRepositoryRejectsDuplicateName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewTableRepository(database.SQL)
	if err := repo.Create("dup", "CREATE TABLE dup(id BIGINT);"); err != nil {
		t.Fatalf("create first: %v", err)
	}
	err = repo.Create("dup", "CREATE TABLE dup(id BIGINT);")
	if err == nil {
		t.Fatalf("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
		!strings.Contains(err.Error(), "already exists") &&
		!strings.Contains(err.Error(), "Duplicate key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
