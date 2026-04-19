package catalog

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCatalogViewCrudFlow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewViewRepository(database.SQL)

	if err := repo.Create("customer_summary", "CREATE VIEW customer_summary AS SELECT 1 AS id;"); err != nil {
		t.Fatalf("create customer_summary: %v", err)
	}
	if err := repo.Create("recent_orders", "CREATE VIEW recent_orders AS SELECT 1 AS id;"); err != nil {
		t.Fatalf("create recent_orders: %v", err)
	}

	views, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("len(views) = %d, want 2", len(views))
	}
	if views[0].Name != "customer_summary" || views[1].Name != "recent_orders" {
		t.Fatalf("unexpected ordering: %#v", views)
	}

	if err := repo.Delete("customer_summary"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	views, err = repo.List()
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(views) != 1 || views[0].Name != "recent_orders" {
		t.Fatalf("unexpected views after delete: %#v", views)
	}
}

func TestCatalogViewRepositoryRejectsBadName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewViewRepository(database.SQL)
	if err := repo.Create("", "CREATE VIEW bad AS SELECT 1"); err == nil {
		t.Fatalf("expected error")
	}
	if err := repo.Create("bad", ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCatalogViewRepositoryRejectsDuplicateName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewViewRepository(database.SQL)
	if err := repo.Create("dup", "CREATE VIEW dup AS SELECT 1"); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := repo.Create("dup", "CREATE VIEW dup AS SELECT 2"); err == nil {
		t.Fatal("expected duplicate error")
	} else if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
		!strings.Contains(err.Error(), "already exists") &&
		!strings.Contains(err.Error(), "Duplicate key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
