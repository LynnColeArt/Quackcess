package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Canvas struct {
	ID        string
	Name      string
	Kind      string
	SpecJSON  string
	Version   int
	SourceRef string
	UpdatedAt time.Time
}

type CanvasRepository struct {
	db *sql.DB
}

func NewCanvasRepository(sqlDB *sql.DB) *CanvasRepository {
	return &CanvasRepository{db: sqlDB}
}

func (r *CanvasRepository) Create(canvas Canvas) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	canvas = normalizeCanvasForStorage(canvas)
	if err := validateCanvasInput(canvas); err != nil {
		return err
	}
	if canvas.Version <= 0 {
		canvas.Version = 1
	}

	const statement = "INSERT INTO quackcess_canvases(id, name, kind, spec_json, version, source_ref) VALUES (?, ?, ?, ?, ?, ?);"
	_, err := r.db.Exec(statement, canvas.ID, canvas.Name, canvas.Kind, canvas.SpecJSON, canvas.Version, nullString(canvas.SourceRef))
	return err
}

func (r *CanvasRepository) List() ([]Canvas, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}

	rows, err := r.db.Query("SELECT id, name, kind, spec_json, COALESCE(version,1), source_ref, updated_at FROM quackcess_canvases ORDER BY name;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var canvases []Canvas
	for rows.Next() {
		var canvas Canvas
		if err := scanCanvasRow(rows, &canvas); err != nil {
			return nil, err
		}
		canvases = append(canvases, canvas)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(canvases, func(i, j int) bool {
		return canvases[i].Name < canvases[j].Name
	})
	return canvases, nil
}

func (r *CanvasRepository) ListByKind(kind string) ([]Canvas, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("catalog repository is not initialized")
	}
	kind = strings.TrimSpace(kind)

	query := "SELECT id, name, kind, spec_json, COALESCE(version,1), source_ref, updated_at FROM quackcess_canvases ORDER BY name;"
	var rows *sql.Rows
	var err error
	if kind == "" {
		rows, err = r.db.Query(query)
	} else {
		rows, err = r.db.Query("SELECT id, name, kind, spec_json, COALESCE(version,1), source_ref, updated_at FROM quackcess_canvases WHERE kind = ? ORDER BY name;", kind)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var canvases []Canvas
	for rows.Next() {
		var canvas Canvas
		if err := scanCanvasRow(rows, &canvas); err != nil {
			return nil, err
		}
		canvases = append(canvases, canvas)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return canvases, nil
}

func (r *CanvasRepository) GetByID(id string) (Canvas, error) {
	if r == nil || r.db == nil {
		return Canvas{}, fmt.Errorf("catalog repository is not initialized")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Canvas{}, fmt.Errorf("canvas id is required")
	}

	var canvas Canvas
	var sourceRef sql.NullString
	err := r.db.QueryRow("SELECT id, name, kind, spec_json, COALESCE(version,1), source_ref, COALESCE(updated_at, CURRENT_TIMESTAMP) FROM quackcess_canvases WHERE id = ? LIMIT 1;", id).Scan(
		&canvas.ID,
		&canvas.Name,
		&canvas.Kind,
		&canvas.SpecJSON,
		&canvas.Version,
		&sourceRef,
		&canvas.UpdatedAt,
	)
	canvas.SourceRef = ""
	if sourceRef.Valid {
		canvas.SourceRef = sourceRef.String
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Canvas{}, fmt.Errorf("canvas not found: %s", id)
		}
		return Canvas{}, err
	}
	return canvas, nil
}

func (r *CanvasRepository) FindByName(name string) (Canvas, error) {
	if r == nil || r.db == nil {
		return Canvas{}, fmt.Errorf("catalog repository is not initialized")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Canvas{}, fmt.Errorf("canvas name is required")
	}

	var canvas Canvas
	var sourceRef sql.NullString
	err := r.db.QueryRow("SELECT id, name, kind, spec_json, COALESCE(version,1), source_ref, COALESCE(updated_at, CURRENT_TIMESTAMP) FROM quackcess_canvases WHERE name = ? ORDER BY name LIMIT 1;", name).Scan(
		&canvas.ID,
		&canvas.Name,
		&canvas.Kind,
		&canvas.SpecJSON,
		&canvas.Version,
		&sourceRef,
		&canvas.UpdatedAt,
	)
	canvas.SourceRef = ""
	if sourceRef.Valid {
		canvas.SourceRef = sourceRef.String
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Canvas{}, fmt.Errorf("canvas not found: %s", name)
		}
		return Canvas{}, err
	}
	return canvas, nil
}

func (r *CanvasRepository) Update(canvas Canvas) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	canvas = normalizeCanvasForStorage(canvas)
	if err := validateCanvasInput(canvas); err != nil {
		return err
	}
	if canvas.Version <= 0 {
		canvas.Version = 1
	}

	result, err := r.db.Exec(
		"UPDATE quackcess_canvases SET name = ?, kind = ?, spec_json = ?, version = ?, source_ref = COALESCE(?, source_ref), updated_at = CURRENT_TIMESTAMP WHERE id = ?;",
		canvas.Name,
		canvas.Kind,
		canvas.SpecJSON,
		canvas.Version,
		nullString(canvas.SourceRef),
		canvas.ID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("canvas not found: %s", canvas.ID)
	}
	return nil
}

func (r *CanvasRepository) Upsert(canvas Canvas) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if err := validateCanvasInput(canvas); err != nil {
		return err
	}
	_, err := r.GetByID(canvas.ID)
	if err == nil {
		return r.Update(canvas)
	}
	if !strings.Contains(err.Error(), "canvas not found") {
		return err
	}
	return r.Create(canvas)
}

func (r *CanvasRepository) Delete(id string) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("catalog repository is not initialized")
	}
	if id == "" {
		return fmt.Errorf("canvas id is required")
	}
	_, err := r.db.Exec("DELETE FROM quackcess_canvases WHERE id = ?;", id)
	return err
}

func validateCanvasInput(canvas Canvas) error {
	canvas = normalizeCanvasForStorage(canvas)
	if canvas.ID == "" {
		return fmt.Errorf("id is required")
	}
	if canvas.Name == "" {
		return fmt.Errorf("name is required")
	}
	if canvas.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if canvas.SpecJSON == "" {
		return fmt.Errorf("spec json is required")
	}
	return nil
}

func normalizeCanvasForStorage(canvas Canvas) Canvas {
	canvas.ID = strings.TrimSpace(canvas.ID)
	canvas.Name = strings.TrimSpace(canvas.Name)
	canvas.Kind = strings.TrimSpace(canvas.Kind)
	canvas.SpecJSON = strings.TrimSpace(canvas.SpecJSON)
	canvas.SourceRef = strings.TrimSpace(canvas.SourceRef)
	return canvas
}

func scanCanvasRow(rows *sql.Rows, canvas *Canvas) error {
	var sourceRef sql.NullString
	if err := rows.Scan(&canvas.ID, &canvas.Name, &canvas.Kind, &canvas.SpecJSON, &canvas.Version, &sourceRef, &canvas.UpdatedAt); err != nil {
		return err
	}
	if sourceRef.Valid {
		canvas.SourceRef = sourceRef.String
	} else {
		canvas.SourceRef = ""
	}
	return nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
