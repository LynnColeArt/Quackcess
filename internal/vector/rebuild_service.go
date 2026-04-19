package vector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type VectorFieldRepository interface {
	GetByID(id string) (VectorField, error)
	Upsert(field VectorField) error
}

type VectorRebuildService struct {
	db           *sql.DB
	repository   VectorFieldRepository
	buildService *VectorBuildService
}

func NewVectorRebuildService(
	database *sql.DB,
	repository VectorFieldRepository,
	buildService *VectorBuildService,
) *VectorRebuildService {
	return &VectorRebuildService{
		db:           database,
		repository:   repository,
		buildService: buildService,
	}
}

func (s *VectorRebuildService) RebuildVector(fieldID string, force bool) (VectorBuildResult, error) {
	return s.RebuildVectorWithProgress(fieldID, force)
}

func (s *VectorRebuildService) RebuildVectorWithProgress(
	fieldID string,
	force bool,
	progressCallbacks ...VectorBuildProgressHandler,
) (VectorBuildResult, error) {
	return s.RebuildVectorWithFilter(fieldID, "", force, progressCallbacks...)
}

func (s *VectorRebuildService) RebuildVectorWithFilter(
	fieldID string,
	filter string,
	force bool,
	progressCallbacks ...VectorBuildProgressHandler,
) (VectorBuildResult, error) {
	if s == nil {
		return VectorBuildResult{}, fmt.Errorf("vector rebuild service is not configured")
	}
	if s.db == nil {
		return VectorBuildResult{}, fmt.Errorf("vector rebuild database is not configured")
	}
	if s.repository == nil {
		return VectorBuildResult{}, fmt.Errorf("vector field repository is not configured")
	}
	if s.buildService == nil {
		return VectorBuildResult{}, fmt.Errorf("vector build service is not configured")
	}
	fieldID = strings.TrimSpace(fieldID)
	if fieldID == "" {
		return VectorBuildResult{}, fmt.Errorf("vector field id is required")
	}

	field, err := s.repository.GetByID(fieldID)
	if err != nil {
		return VectorBuildResult{}, err
	}
	sourceTexts, sourceUpdatedAt, err := s.extractSourceTexts(context.Background(), field.TableName, field.SourceColumn, filter)
	if err != nil {
		return VectorBuildResult{}, err
	}

	result, err := s.buildService.BuildFromSourceTexts(
		context.Background(),
		field,
		sourceTexts,
		sourceUpdatedAt,
		force,
		0,
		progressCallbacks...,
	)
	if err != nil {
		return VectorBuildResult{}, err
	}
	if !result.Built {
		return result, nil
	}

	if err := s.persistVectors(context.Background(), field.TableName, field.VectorColumn, result.VectorsByID); err != nil {
		return VectorBuildResult{}, err
	}
	if sourceUpdatedAt.After(result.Field.SourceLastUpdatedAt) {
		result.Field.SourceLastUpdatedAt = sourceUpdatedAt
	}
	if err := s.repository.Upsert(result.Field); err != nil {
		return VectorBuildResult{}, err
	}

	return result, nil
}

func (s *VectorRebuildService) extractSourceTexts(
	ctx context.Context,
	tableName,
	sourceColumn string,
	filter string,
) (map[string]string, time.Time, error) {
	if strings.TrimSpace(tableName) == "" {
		return nil, time.Time{}, fmt.Errorf("vector tableName is required")
	}
	if strings.TrimSpace(sourceColumn) == "" {
		return nil, time.Time{}, fmt.Errorf("vector source column is required")
	}

	columns, err := s.fetchTableColumns(ctx, tableName)
	if err != nil {
		return nil, time.Time{}, err
	}
	if _, ok := columns[normalizeIdentifier(sourceColumn)]; !ok {
		return nil, time.Time{}, fmt.Errorf("source column not found: %s", sourceColumn)
	}
	sourceTextQuery := fmt.Sprintf(
		`SELECT rowid, %s FROM %s WHERE %s IS NOT NULL`,
		quoteIdentifier(sourceColumn),
		quoteIdentifier(tableName),
		quoteIdentifier(sourceColumn),
	)
	filter = strings.TrimSpace(filter)
	if filter != "" {
		sourceTextQuery += " AND (" + filter + ")"
	}
	rows, err := s.db.QueryContext(ctx, sourceTextQuery)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	values := make(map[string]string)
	for rows.Next() {
		var rowID int64
		var sourceTextValue any
		if err := rows.Scan(&rowID, &sourceTextValue); err != nil {
			return nil, time.Time{}, err
		}
		sourceText := normalizeVectorSourceText(sourceTextValue)
		if sourceText == "" {
			continue
		}
		values[strconv.FormatInt(rowID, 10)] = sourceText
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, err
	}

	updatedAtColumn := firstAvailableColumn(columns, []string{"updated_at", "modified_at", "source_updated_at"})
	var sourceUpdatedAt time.Time
	if updatedAtColumn != "" {
		row := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT MAX(%s) FROM %s`, quoteIdentifier(updatedAtColumn), quoteIdentifier(tableName)))
		var maxUpdated sql.NullTime
		if err := row.Scan(&maxUpdated); err != nil {
			return nil, time.Time{}, err
		}
		if maxUpdated.Valid {
			sourceUpdatedAt = maxUpdated.Time
		}
	}

	return values, sourceUpdatedAt, nil
}

type tableColumn struct {
	name string
}

func (s *VectorRebuildService) fetchTableColumns(ctx context.Context, tableName string) (map[string]tableColumn, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, quoteIdentifier(tableName)))
	if err != nil {
		return nil, fmt.Errorf("failed to read table columns for %s: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]tableColumn)
	for rows.Next() {
		var id int
		var name string
		var dataType, notNull, defaultValue, pk any
		if err := rows.Scan(&id, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		key := normalizeIdentifier(name)
		columns[key] = tableColumn{name: name}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table not found: %s", tableName)
	}
	return columns, nil
}

func firstAvailableColumn(columns map[string]tableColumn, candidates []string) string {
	for _, candidate := range candidates {
		key := normalizeIdentifier(candidate)
		if info, ok := columns[key]; ok && info.name != "" {
			return info.name
		}
	}
	return ""
}

func (s *VectorRebuildService) persistVectors(ctx context.Context, tableName, vectorColumn string, vectorsByID map[string][]float64) error {
	columns, err := s.fetchTableColumns(ctx, tableName)
	if err != nil {
		return err
	}
	if _, ok := columns[normalizeIdentifier(vectorColumn)]; !ok {
		return fmt.Errorf("vector column not found: %s", vectorColumn)
	}
	if len(vectorsByID) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for id, values := range vectorsByID {
		vectorText, encodeErr := vectorToJSON(values)
		if encodeErr != nil {
			return encodeErr
		}
		updateAsArray := fmt.Sprintf(`UPDATE %s SET %s = CAST(? AS FLOAT[]) WHERE rowid = ?`, quoteIdentifier(tableName), quoteIdentifier(vectorColumn))
		if _, err := tx.ExecContext(ctx, updateAsArray, vectorText, id); err != nil {
			updateAsText := fmt.Sprintf(`UPDATE %s SET %s = ? WHERE rowid = ?`, quoteIdentifier(tableName), quoteIdentifier(vectorColumn))
			if _, fallbackErr := tx.ExecContext(ctx, updateAsText, vectorText, id); fallbackErr != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to persist vector for row %s: %w", id, fallbackErr)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func normalizeIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeVectorSourceText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func vectorToJSON(values []float64) (string, error) {
	payload := make([]float64, len(values))
	copy(payload, values)
	output, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
