package catalog

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/db"
)

func TestCatalogRelationshipCrudFlow(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	tableRepo := NewTableRepository(database.SQL)
	if err := tableRepo.Create("customers", "CREATE TABLE customers(id BIGINT PRIMARY KEY, name TEXT);"); err != nil {
		t.Fatalf("create customers table: %v", err)
	}
	if err := tableRepo.Create("orders", "CREATE TABLE orders(id BIGINT PRIMARY KEY, customer_id BIGINT);"); err != nil {
		t.Fatalf("create orders table: %v", err)
	}

	repo := NewRelationshipRepository(database.SQL)
	if err := repo.Create(Relationship{
		Name:       "orders.customer_id_to_customers.id",
		FromTable:  "orders",
		FromColumn: "customer_id",
		ToTable:    "customers",
		ToColumn:   "id",
	}); err != nil {
		t.Fatalf("create relationship: %v", err)
	}

	rels, err := repo.List()
	if err != nil {
		t.Fatalf("list relationships: %v", err)
	}
	if len(rels) != 1 || rels[0].Name != "orders.customer_id_to_customers.id" {
		t.Fatalf("unexpected relationships: %#v", rels)
	}

	if err := repo.Delete("orders.customer_id_to_customers.id"); err != nil {
		t.Fatalf("delete relationship: %v", err)
	}

	rels, err = repo.List()
	if err != nil {
		t.Fatalf("list relationships after delete: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("expected empty relationships, got %#v", rels)
	}
}

func TestCatalogRelationshipRepositoryRejectsBadInput(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	repo := NewRelationshipRepository(database.SQL)
	if err := repo.Create(Relationship{}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestCatalogRelationshipRepositoryRejectsDuplicateName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "meta.duckdb")

	database, err := db.Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	tableRepo := NewTableRepository(database.SQL)
	if err := tableRepo.Create("users", "CREATE TABLE users(id BIGINT PRIMARY KEY);"); err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if err := tableRepo.Create("sessions", "CREATE TABLE sessions(id BIGINT PRIMARY KEY, user_id BIGINT);"); err != nil {
		t.Fatalf("create sessions table: %v", err)
	}

	repo := NewRelationshipRepository(database.SQL)
	rel := Relationship{
		Name:       "sessions.user_id",
		FromTable:  "sessions",
		FromColumn: "user_id",
		ToTable:    "users",
		ToColumn:   "id",
	}
	if err := repo.Create(rel); err != nil {
		t.Fatalf("create first relationship: %v", err)
	}
	err = repo.Create(rel)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "UNIQUE constraint failed") &&
		!strings.Contains(err.Error(), "already exists") &&
		!strings.Contains(err.Error(), "Duplicate key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
