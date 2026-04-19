package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/vector"
)

type VectorFieldRepository struct {
	db *sql.DB
}

func NewVectorFieldRepository(sqlDB *sql.DB) *VectorFieldRepository {
	return &VectorFieldRepository{db: sqlDB}
}

func (r *VectorFieldRepository) Create(field vector.VectorField) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	field, err := vector.CanonicalizeVectorField(field)
	if err != nil {
		return err
	}
	if field.SchemaVersion == "" {
		field.SchemaVersion = vector.CurrentVectorFieldSchemaVersion()
	}
	const statement = `
INSERT INTO quackcess_vector_fields(
	id,
	schema_version,
	table_name,
	source_column,
	vector_column,
	dimension,
	provider,
	model,
	stale_after_hours,
	last_indexed_at,
	source_last_updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	_, err = r.db.Exec(
		statement,
		field.ID,
		field.SchemaVersion,
		field.TableName,
		field.SourceColumn,
		field.VectorColumn,
		field.Dimension,
		field.Provider,
		field.Model,
		field.StaleAfterHours,
		nullTime(field.LastIndexedAt),
		nullTime(field.SourceLastUpdatedAt),
	)
	return err
}

func (r *VectorFieldRepository) GetByID(id string) (vector.VectorField, error) {
	if r == nil || r.db == nil {
		return vector.VectorField{}, fmt.Errorf("catalog repository is not initialized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return vector.VectorField{}, fmt.Errorf("vector field id is required")
	}

	var field vector.VectorField
	var lastIndexed sql.NullTime
	var sourceUpdated sql.NullTime
	err := r.db.QueryRow(`
SELECT
	id,
	schema_version,
	table_name,
	source_column,
	vector_column,
	dimension,
	provider,
	model,
	stale_after_hours,
	last_indexed_at,
	source_last_updated_at
FROM quackcess_vector_fields WHERE id = ? LIMIT 1`, id).Scan(
		&field.ID,
		&field.SchemaVersion,
		&field.TableName,
		&field.SourceColumn,
		&field.VectorColumn,
		&field.Dimension,
		&field.Provider,
		&field.Model,
		&field.StaleAfterHours,
		&lastIndexed,
		&sourceUpdated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return vector.VectorField{}, fmt.Errorf("vector field not found: %s", id)
		}
		return vector.VectorField{}, err
	}
	field.LastIndexedAt = timeFromNullTime(lastIndexed)
	field.SourceLastUpdatedAt = timeFromNullTime(sourceUpdated)
	return field, nil
}

func (r *VectorFieldRepository) List() ([]vector.VectorField, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}
	rows, err := r.db.Query(`
SELECT
	id,
	schema_version,
	table_name,
	source_column,
	vector_column,
	dimension,
	provider,
	model,
	stale_after_hours,
	last_indexed_at,
	source_last_updated_at
FROM quackcess_vector_fields ORDER BY table_name, source_column`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := make([]vector.VectorField, 0)
	for rows.Next() {
		var field vector.VectorField
		var lastIndexed sql.NullTime
		var sourceUpdated sql.NullTime
		if err := rows.Scan(
			&field.ID,
			&field.SchemaVersion,
			&field.TableName,
			&field.SourceColumn,
			&field.VectorColumn,
			&field.Dimension,
			&field.Provider,
			&field.Model,
			&field.StaleAfterHours,
			&lastIndexed,
			&sourceUpdated,
		); err != nil {
			return nil, err
		}
		field.LastIndexedAt = timeFromNullTime(lastIndexed)
		field.SourceLastUpdatedAt = timeFromNullTime(sourceUpdated)
		fields = append(fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func (r *VectorFieldRepository) Upsert(field vector.VectorField) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	field, err := vector.CanonicalizeVectorField(field)
	if err != nil {
		return err
	}
	if field.SchemaVersion == "" {
		field.SchemaVersion = vector.CurrentVectorFieldSchemaVersion()
	}

	_, err = r.db.Exec(`
INSERT INTO quackcess_vector_fields(
	id,
	schema_version,
	table_name,
	source_column,
	vector_column,
	dimension,
	provider,
	model,
	stale_after_hours,
	last_indexed_at,
	source_last_updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
	schema_version = excluded.schema_version,
	vector_column = excluded.vector_column,
	dimension = excluded.dimension,
	provider = excluded.provider,
	model = excluded.model,
	stale_after_hours = excluded.stale_after_hours,
	last_indexed_at = excluded.last_indexed_at,
	source_last_updated_at = excluded.source_last_updated_at;`,
		field.ID,
		field.SchemaVersion,
		field.TableName,
		field.SourceColumn,
		field.VectorColumn,
		field.Dimension,
		field.Provider,
		field.Model,
		field.StaleAfterHours,
		nullTime(field.LastIndexedAt),
		nullTime(field.SourceLastUpdatedAt),
	)

	return err
}

func (r *VectorFieldRepository) Delete(id string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("vector field id is required")
	}
	_, err := r.db.Exec("DELETE FROM quackcess_vector_fields WHERE id = ?;", id)
	return err
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func timeFromNullTime(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
