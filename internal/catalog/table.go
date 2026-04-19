package catalog

import (
	"database/sql"
	"fmt"
	"sort"
)

type Table struct {
	Name string
	SQL  string
}

type TableRepository struct {
	db *sql.DB
}

func NewTableRepository(sqlDB *sql.DB) *TableRepository {
	return &TableRepository{db: sqlDB}
}

func (r *TableRepository) Create(name string, sqlText string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}
	const statement = "INSERT INTO quackcess_tables(name, sql_text) VALUES (?, ?);"
	_, err := r.db.Exec(statement, name, sqlText)
	return err
}

func (r *TableRepository) List() ([]Table, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}

	rows, err := r.db.Query("SELECT name, sql_text FROM quackcess_tables ORDER BY name;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var table Table
		if err := rows.Scan(&table.Name, &table.SQL); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})
	return tables, nil
}

func (r *TableRepository) Delete(name string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	_, err := r.db.Exec("DELETE FROM quackcess_tables WHERE name = ?;", name)
	return err
}
