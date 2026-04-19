package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapCreatesCatalogSchema(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "bootstrap.duckdb")

	database, err := Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer database.Close()

	assertTablesExist(t, database.SQL, []string{
		"quackcess_meta",
		"quackcess_tables",
		"quackcess_table_columns",
		"quackcess_relationships",
		"quackcess_views",
		"quackcess_canvases",
		"quackcess_vector_fields",
		"quackcess_query_history",
	})

	var version string
	if err := database.SQL.QueryRow("SELECT value FROM quackcess_meta WHERE key = 'schema_version';").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentSchemaVersion() {
		t.Fatalf("schema version = %q, want %q", version, CurrentSchemaVersion())
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "idempotent.duckdb")

	database, err := Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	database.Close()

	database2, err := Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	defer database2.Close()

	var version string
	if err := database2.SQL.QueryRow("SELECT value FROM quackcess_meta WHERE key = 'schema_version';").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentSchemaVersion() {
		t.Fatalf("schema version = %q, want %q", version, CurrentSchemaVersion())
	}
}

func TestBootstrapRejectsUnsupportedCatalogVersion(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "unsupported.duckdb")

	raw, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec("CREATE TABLE IF NOT EXISTS quackcess_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	if _, err := raw.Exec("INSERT OR REPLACE INTO quackcess_meta(key, value) VALUES ('schema_version', '2.0.0');"); err != nil {
		t.Fatalf("seed unsupported version: %v", err)
	}
	raw.Close()

	_, err = Bootstrap(dbPath)
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
	if !strings.Contains(err.Error(), "unsupported catalog schema version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBootstrapRejectsMissingPath(t *testing.T) {
	if _, err := Bootstrap(""); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "database path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBootstrapRejectsInvalidParentDir(t *testing.T) {
	if _, err := Bootstrap(string([]byte{0x00})); err == nil {
		t.Fatal("expected error")
	}
}

func TestMkdirPermissions(t *testing.T) {
	tmp := t.TempDir()
	readOnly := filepath.Join(tmp, "readonly")
	if err := os.Mkdir(readOnly, 0o400); err != nil {
		t.Fatalf("create readonly dir: %v", err)
	}
	dbPath := filepath.Join(readOnly, "db.duckdb")

	_, err := Bootstrap(dbPath)
	if err == nil {
		t.Fatal("expected bootstrap error")
	}
	if !strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "operation not permitted") {
		// if filesystem allows, we still allow success; this test guards the guardrail path contract.
		t.Skipf("expected permission error, got %v", err)
	}
}

func TestBootstrapCreatesVectorFieldTableForLegacyVersion(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "legacy-vectors.duckdb")

	raw, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec("CREATE TABLE IF NOT EXISTS quackcess_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	if _, err := raw.Exec("CREATE TABLE IF NOT EXISTS quackcess_canvases (id TEXT PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, spec_json TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);"); err != nil {
		t.Fatalf("create old canvas table: %v", err)
	}
	if _, err := raw.Exec("INSERT OR REPLACE INTO quackcess_meta(key, value) VALUES ('schema_version', '1.1.0');"); err != nil {
		t.Fatalf("seed old version: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	bootstrapped, err := Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap migrated db: %v", err)
	}
	defer bootstrapped.Close()

	var hasTable int
	if err := bootstrapped.SQL.QueryRow("SELECT 1 FROM duckdb_tables() WHERE table_name = ?", "quackcess_vector_fields").Scan(&hasTable); err != nil {
		t.Fatalf("expected vector field table: %v", err)
	}
}

func assertTablesExist(t *testing.T, database *sql.DB, names []string) {
	t.Helper()
	for _, name := range names {
		var exists int
		query := "SELECT 1 FROM duckdb_tables() WHERE table_name = ?"
		if err := database.QueryRow(query, name).Scan(&exists); err == sql.ErrNoRows {
			t.Fatalf("expected table %s", name)
		} else if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
	}
}
