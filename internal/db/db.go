package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
)

const (
	currentSchemaVersion    = "1.2.0"
	migratedCanvasVersion  = "1.1.0"
	previousSchemaVersion  = "1.0.0"
)

const (
	createMetaTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

	createTablesTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_tables (
	name TEXT PRIMARY KEY,
	sql_text TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	createColumnsTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_table_columns (
	table_name TEXT NOT NULL,
	name TEXT NOT NULL,
	position INTEGER NOT NULL,
	data_type TEXT NOT NULL,
	is_nullable BOOLEAN NOT NULL DEFAULT TRUE,
	is_primary_key BOOLEAN NOT NULL DEFAULT FALSE,
	default_sql TEXT,
	description TEXT,
	PRIMARY KEY (table_name, name),
	UNIQUE (table_name, position),
	FOREIGN KEY (table_name) REFERENCES quackcess_tables(name)
);`

	createRelationshipsTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_relationships (
	name TEXT PRIMARY KEY,
	from_table TEXT NOT NULL,
	from_column TEXT NOT NULL,
	to_table TEXT NOT NULL,
	to_column TEXT NOT NULL,
	on_delete TEXT NOT NULL DEFAULT 'NO ACTION',
	on_update TEXT NOT NULL DEFAULT 'NO ACTION',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	createViewsTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_views (
	name TEXT PRIMARY KEY,
	sql_text TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

createCanvasesTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_canvases (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	kind TEXT NOT NULL,
	spec_json TEXT NOT NULL,
	version INTEGER NOT NULL DEFAULT 1,
	source_ref TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	createVectorFieldsTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_vector_fields (
	id TEXT PRIMARY KEY,
	schema_version TEXT NOT NULL DEFAULT '1.0.0',
	table_name TEXT NOT NULL,
	source_column TEXT NOT NULL,
	vector_column TEXT NOT NULL,
	dimension INTEGER NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	stale_after_hours INTEGER NOT NULL DEFAULT 24,
	last_indexed_at TIMESTAMP,
	source_last_updated_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (table_name, source_column)
);`

	createQueryHistoryTableSQL = `
CREATE TABLE IF NOT EXISTS quackcess_query_history (
	executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	sql_text TEXT NOT NULL,
	parameters_json TEXT,
	row_count INTEGER NOT NULL DEFAULT 0,
	duration_milliseconds BIGINT NOT NULL DEFAULT 0,
	success BOOLEAN NOT NULL,
	error_text TEXT
);`

	insertDefaultVersionSQL = `
INSERT INTO quackcess_meta(key, value)
VALUES ('schema_version', ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value;`

	selectSchemaVersionSQL = "SELECT value FROM quackcess_meta WHERE key = 'schema_version';"
)

type DB struct {
	Path string
	SQL  *sql.DB
}

type DBError struct {
	Op  string
	Err error
}

func (e *DBError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("db %s: %v", e.Op, e.Err)
}

func (e *DBError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func CurrentSchemaVersion() string {
	return currentSchemaVersion
}

// Bootstrap opens a duckdb file and ensures the project catalog schema exists.
func Bootstrap(path string) (*DB, error) {
	if path == "" {
		return nil, &DBError{Op: "bootstrap", Err: errors.New("database path is required")}
	}
	if err := ensureParentDir(path); err != nil {
		return nil, &DBError{Op: "bootstrap", Err: err}
	}

	sqlDB, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, normalizeError("bootstrap", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, normalizeError("bootstrap", err)
	}

	database := &DB{Path: path, SQL: sqlDB}
	if err := database.ensureSchema(); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// ensureSchema guarantees catalog tables are available and migrations are applied.
func (d *DB) ensureSchema() error {
	version, err := d.getSchemaVersion()
	if err != nil {
		return err
	}

	switch version {
	case "", "0.0.0":
		if err := d.createBaseSchema(); err != nil {
			return err
		}
		if err := d.setSchemaVersion(currentSchemaVersion); err != nil {
			return err
		}
		return nil
	case currentSchemaVersion:
		if err := d.createBaseSchema(); err != nil {
			return err
		}
		return nil
	case previousSchemaVersion:
		if err := d.createBaseSchema(); err != nil {
			return err
		}
		if err := d.migrateCanvasesTo11(); err != nil {
			return err
		}
		if err := d.setSchemaVersion(currentSchemaVersion); err != nil {
			return err
		}
		return nil
	case migratedCanvasVersion:
		if err := d.createBaseSchema(); err != nil {
			return err
		}
		if err := d.setSchemaVersion(currentSchemaVersion); err != nil {
			return err
		}
		return nil
	default:
		return &DBError{Op: "migrate", Err: fmt.Errorf("unsupported catalog schema version: %s", version)}
	}
}

func (d *DB) createBaseSchema() error {
	if _, err := d.SQL.Exec(createMetaTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createTablesTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createColumnsTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createRelationshipsTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createViewsTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createCanvasesTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createVectorFieldsTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	if _, err := d.SQL.Exec(createQueryHistoryTableSQL); err != nil {
		return normalizeError("create schema", err)
	}
	return nil
}

func (d *DB) getSchemaVersion() (string, error) {
	if _, err := d.SQL.Exec(createMetaTableSQL); err != nil {
		return "", normalizeError("read schema version", err)
	}

	var version string
	err := d.SQL.QueryRow(selectSchemaVersionSQL).Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", normalizeError("read schema version", err)
	}
	return version, nil
}

func (d *DB) setSchemaVersion(version string) error {
	if _, err := d.SQL.Exec(insertDefaultVersionSQL, version); err != nil {
		return normalizeError("write schema version", err)
	}
	return nil
}

func (d *DB) migrateCanvasesTo11() error {
	columns, err := d.listCanvasesColumns()
	if err != nil {
		return err
	}

	needsFill := false

	if _, found := columns["version"]; !found {
		if _, err := d.SQL.Exec("ALTER TABLE quackcess_canvases ADD COLUMN version INTEGER;"); err != nil {
			return normalizeError("migrate canvases", err)
		}
		needsFill = true
	}
	if _, found := columns["source_ref"]; !found {
		if _, err := d.SQL.Exec("ALTER TABLE quackcess_canvases ADD COLUMN source_ref TEXT;"); err != nil {
			return normalizeError("migrate canvases", err)
		}
	}
	if _, found := columns["updated_at"]; !found {
		if _, err := d.SQL.Exec("ALTER TABLE quackcess_canvases ADD COLUMN updated_at TIMESTAMP;"); err != nil {
			return normalizeError("migrate canvases", err)
		}
		needsFill = true
	}

	if needsFill {
		if _, err := d.SQL.Exec("UPDATE quackcess_canvases SET version = COALESCE(version, 1), updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP);"); err != nil {
			return normalizeError("migrate canvases", err)
		}
	}

	return nil
}

func (d *DB) listCanvasesColumns() (map[string]struct{}, error) {
	rows, err := d.SQL.Query("PRAGMA table_info('quackcess_canvases')")
	if err != nil {
		return nil, normalizeError("read schema", err)
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var cid any
		var name string
		var columnType any
		var notnull any
		var defaultValue any
		var pk any
		if err := rows.Scan(&cid, &name, &columnType, &notnull, &defaultValue, &pk); err != nil {
			return nil, normalizeError("read schema", err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeError("read schema", err)
	}
	return columns, nil
}

func (d *DB) Close() error {
	if d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}

func normalizeError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &DBError{Op: op, Err: err}
}
