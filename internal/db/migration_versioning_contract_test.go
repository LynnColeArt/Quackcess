package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCurrentSchemaVersionFormat(t *testing.T) {
	version := CurrentSchemaVersion()
	if version == "" {
		t.Fatal("expected non-empty schema version")
	}
}

func TestBootstrapMigratesCanvasMetadataColumns(t *testing.T) {
	tmp := t.TempDir()
	dbPath := tmp + "/migration-canvas.duckdb"

	raw, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

	if _, err := raw.Exec("CREATE TABLE IF NOT EXISTS quackcess_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	if _, err := raw.Exec(`
CREATE TABLE IF NOT EXISTS quackcess_canvases (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	kind TEXT NOT NULL,
	spec_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		t.Fatalf("create old canvases table: %v", err)
	}
	if _, err := raw.Exec("INSERT OR REPLACE INTO quackcess_meta(key, value) VALUES ('schema_version', '1.0.0');"); err != nil {
		t.Fatalf("seed old version: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO quackcess_canvases(id, name, kind, spec_json) VALUES ('c1', 'old', 'query', '{"nodes":[],"edges":[]}');`); err != nil {
		t.Fatalf("seed old canvas: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	bootstrapped, err := Bootstrap(dbPath)
	if err != nil {
		t.Fatalf("bootstrap migrated db: %v", err)
	}
	defer bootstrapped.Close()

	var version string
	if err := bootstrapped.SQL.QueryRow("SELECT value FROM quackcess_meta WHERE key = 'schema_version';").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentSchemaVersion() {
		t.Fatalf("schema version = %q, want %q", version, CurrentSchemaVersion())
	}

	var migratedVersion int
	if err := bootstrapped.SQL.QueryRow("SELECT version FROM quackcess_canvases WHERE id='c1';").Scan(&migratedVersion); err != nil {
		t.Fatalf("read migrated canvas version: %v", err)
	}
	if migratedVersion != 1 {
		t.Fatalf("migrated canvas version = %d, want 1", migratedVersion)
	}

	columns, err := listCanvasesColumnsForTest(bootstrapped.SQL)
	if err != nil {
		t.Fatalf("read canvas columns: %v", err)
	}
	if _, ok := columns["source_ref"]; !ok {
		t.Fatalf("expected source_ref column after migration")
	}
	if _, ok := columns["updated_at"]; !ok {
		t.Fatalf("expected updated_at column after migration")
	}

	if _, ok := columns["version"]; !ok {
		t.Fatalf("expected version column after migration")
	}
}

func TestBootstrapMigratesVectorMetadataTableForLegacyVersion(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "migration-vector-field.duckdb")

	raw, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.Exec("CREATE TABLE IF NOT EXISTS quackcess_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);"); err != nil {
		t.Fatalf("create meta table: %v", err)
	}
	if _, err := raw.Exec("CREATE TABLE IF NOT EXISTS quackcess_canvases (id TEXT PRIMARY KEY, name TEXT NOT NULL, kind TEXT NOT NULL, spec_json TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);"); err != nil {
		t.Fatalf("create old canvases table: %v", err)
	}
	if _, err := raw.Exec("INSERT OR REPLACE INTO quackcess_meta(key, value) VALUES ('schema_version', '1.0.0');"); err != nil {
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

	var version string
	if err := bootstrapped.SQL.QueryRow("SELECT value FROM quackcess_meta WHERE key = 'schema_version';").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentSchemaVersion() {
		t.Fatalf("schema version = %q, want %q", version, CurrentSchemaVersion())
	}

	var hasTable int
	if err := bootstrapped.SQL.QueryRow("SELECT 1 FROM duckdb_tables() WHERE table_name = ?", "quackcess_vector_fields").Scan(&hasTable); err != nil {
		t.Fatalf("expected vector field table after migration: %v", err)
	}
}

func listCanvasesColumnsForTest(database *sql.DB) (map[string]struct{}, error) {
	rows, err := database.Query("PRAGMA table_info('quackcess_canvases')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var cid any
		var name string
		var colType any
		var notNull any
		var defaultValue any
		var pk any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = struct{}{}
	}
	return columns, rows.Err()
}
