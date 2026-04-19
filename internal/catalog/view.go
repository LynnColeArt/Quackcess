package catalog

import (
	"database/sql"
	"fmt"
	"sort"
)

type View struct {
	Name string
	SQL  string
}

type ViewRepository struct {
	db *sql.DB
}

func NewViewRepository(sqlDB *sql.DB) *ViewRepository {
	return &ViewRepository{db: sqlDB}
}

func (r *ViewRepository) Create(name string, sqlText string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if sqlText == "" {
		return fmt.Errorf("sql is required")
	}

	const statement = "INSERT INTO quackcess_views(name, sql_text) VALUES (?, ?);"
	_, err := r.db.Exec(statement, name, sqlText)
	return err
}

func (r *ViewRepository) List() ([]View, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}

	rows, err := r.db.Query("SELECT name, sql_text FROM quackcess_views")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []View
	for rows.Next() {
		var view View
		if err := rows.Scan(&view.Name, &view.SQL); err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(views, func(i, j int) bool {
		return views[i].Name < views[j].Name
	})
	return views, nil
}

func (r *ViewRepository) Delete(name string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}
	_, err := r.db.Exec("DELETE FROM quackcess_views WHERE name = ?;", name)
	return err
}
