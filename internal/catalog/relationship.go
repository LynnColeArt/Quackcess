package catalog

import (
	"database/sql"
	"fmt"
	"sort"
)

type Relationship struct {
	Name       string
	FromTable  string
	FromColumn string
	ToTable    string
	ToColumn   string
	OnDelete   string
	OnUpdate   string
}

type RelationshipRepository struct {
	db *sql.DB
}

func NewRelationshipRepository(sqlDB *sql.DB) *RelationshipRepository {
	return &RelationshipRepository{db: sqlDB}
}

func (r *RelationshipRepository) Create(relationship Relationship) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	normalized, err := normalizeRelationshipInput(relationship)
	if err != nil {
		return err
	}

	const statement = `INSERT INTO quackcess_relationships(
		name,
		from_table,
		from_column,
		to_table,
		to_column,
		on_delete,
		on_update
	) VALUES (?, ?, ?, ?, ?, ?, ?);`

	_, err = r.db.Exec(
		statement,
		normalized.Name,
		normalized.FromTable,
		normalized.FromColumn,
		normalized.ToTable,
		normalized.ToColumn,
		normalized.OnDelete,
		normalized.OnUpdate,
	)
	return err
}

func (r *RelationshipRepository) List() ([]Relationship, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}

	rows, err := r.db.Query(`
	SELECT
		name,
		from_table,
		from_column,
		to_table,
		to_column,
		on_delete,
		on_update
	FROM quackcess_relationships
	ORDER BY name;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []Relationship
	for rows.Next() {
		var relationship Relationship
		if err := rows.Scan(
			&relationship.Name,
			&relationship.FromTable,
			&relationship.FromColumn,
			&relationship.ToTable,
			&relationship.ToColumn,
			&relationship.OnDelete,
			&relationship.OnUpdate,
		); err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(relationships, func(i, j int) bool {
		return relationships[i].Name < relationships[j].Name
	})
	return relationships, nil
}

func (r *RelationshipRepository) Delete(name string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if name == "" {
		return fmt.Errorf("relationship name is required")
	}
	_, err := r.db.Exec("DELETE FROM quackcess_relationships WHERE name = ?;", name)
	return err
}

func normalizeRelationshipInput(r Relationship) (Relationship, error) {
	if r.Name == "" {
		return Relationship{}, fmt.Errorf("relationship name is required")
	}
	if r.FromTable == "" {
		return Relationship{}, fmt.Errorf("from table is required")
	}
	if r.FromColumn == "" {
		return Relationship{}, fmt.Errorf("from column is required")
	}
	if r.ToTable == "" {
		return Relationship{}, fmt.Errorf("to table is required")
	}
	if r.ToColumn == "" {
		return Relationship{}, fmt.Errorf("to column is required")
	}
	if r.OnDelete == "" {
		r.OnDelete = "NO ACTION"
	}
	if r.OnUpdate == "" {
		r.OnUpdate = "NO ACTION"
	}
	return r, nil
}
