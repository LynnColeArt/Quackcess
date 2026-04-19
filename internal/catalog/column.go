package catalog

import (
	"database/sql"
	"fmt"
	"sort"
)

type Column struct {
	TableName    string
	Name         string
	Position     int
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
	DefaultSQL   string
	Description  string
}

type ColumnRepository struct {
	db *sql.DB
}

func NewColumnRepository(sqlDB *sql.DB) *ColumnRepository {
	return &ColumnRepository{db: sqlDB}
}

func (r *ColumnRepository) Create(column Column) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if err := validateColumnInput(column); err != nil {
		return err
	}

	const statement = `INSERT INTO quackcess_table_columns(
		table_name,
		name,
		position,
		data_type,
		is_nullable,
		is_primary_key,
		default_sql,
		description
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?);`

	_, err := r.db.Exec(
		statement,
		column.TableName,
		column.Name,
		column.Position,
		column.DataType,
		column.IsNullable,
		column.IsPrimaryKey,
		column.DefaultSQL,
		column.Description,
	)
	return err
}

func (r *ColumnRepository) List(tableName string) ([]Column, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}
	if tableName == "" {
		return nil, fmt.Errorf("table name is required")
	}

	rows, err := r.db.Query(`
	SELECT
		table_name,
		name,
		position,
		data_type,
		is_nullable,
		is_primary_key,
		default_sql,
		description
	FROM quackcess_table_columns
	WHERE table_name = ?
	ORDER BY position;
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(
			&c.TableName,
			&c.Name,
			&c.Position,
			&c.DataType,
			&c.IsNullable,
			&c.IsPrimaryKey,
			&c.DefaultSQL,
			&c.Description,
		); err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(columns, func(i, j int) bool {
		return columns[i].Position < columns[j].Position
	})
	return columns, nil
}

func (r *ColumnRepository) Delete(tableName string, name string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if tableName == "" {
		return fmt.Errorf("table name is required")
	}
	if name == "" {
		return fmt.Errorf("column name is required")
	}

	_, err := r.db.Exec("DELETE FROM quackcess_table_columns WHERE table_name = ? AND name = ?;", tableName, name)
	return err
}

func validateColumnInput(c Column) error {
	if c.TableName == "" {
		return fmt.Errorf("table name is required")
	}
	if c.Name == "" {
		return fmt.Errorf("column name is required")
	}
	if c.DataType == "" {
		return fmt.Errorf("data type is required")
	}
	if c.Position <= 0 {
		return fmt.Errorf("position must be greater than 0")
	}
	return nil
}
