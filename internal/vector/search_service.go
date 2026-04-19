package vector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type VectorSearchResult struct {
	Field   VectorField
	Matches []SimilarityMatch
}

type VectorSearchService struct {
	db           *sql.DB
	repository   VectorFieldRepository
	buildService *VectorBuildService
}

func NewVectorSearchService(
	database *sql.DB,
	repository VectorFieldRepository,
	buildService *VectorBuildService,
) *VectorSearchService {
	return &VectorSearchService{
		db:           database,
		repository:   repository,
		buildService: buildService,
	}
}

func (s *VectorSearchService) SearchByFieldID(
	ctx context.Context,
	fieldID string,
	queryText string,
	limit int,
) (VectorSearchResult, error) {
	if s == nil {
		return VectorSearchResult{}, fmt.Errorf("vector search service is not configured")
	}
	if s.db == nil {
		return VectorSearchResult{}, fmt.Errorf("vector search database is not configured")
	}
	if s.repository == nil {
		return VectorSearchResult{}, fmt.Errorf("vector field repository is not configured")
	}
	if s.buildService == nil {
		return VectorSearchResult{}, fmt.Errorf("vector build service is not configured")
	}
	fieldID = strings.TrimSpace(fieldID)
	if fieldID == "" {
		return VectorSearchResult{}, fmt.Errorf("vector field id is required")
	}
	if strings.TrimSpace(queryText) == "" {
		return VectorSearchResult{}, fmt.Errorf("query text is required")
	}
	if limit <= 0 {
		limit = 10
	}

	field, err := s.repository.GetByID(fieldID)
	if err != nil {
		return VectorSearchResult{}, err
	}
	provider, err := s.buildService.ResolveProvider(field)
	if err != nil {
		return VectorSearchResult{}, err
	}

	query, err := parseVectorColumnValues(ctx, s.db, field.TableName, field.VectorColumn)
	if err != nil {
		return VectorSearchResult{}, err
	}
	matches, err := SearchByText(ctx, provider, queryText, query, limit)
	if err != nil {
		return VectorSearchResult{}, err
	}

	return VectorSearchResult{
		Field:   field,
		Matches: matches,
	}, nil
}

func parseVectorColumnValues(ctx context.Context, database *sql.DB, tableName, vectorColumn string) (map[string][]float64, error) {
	if strings.TrimSpace(tableName) == "" {
		return nil, fmt.Errorf("vector tableName is required")
	}
	vectorColumn = strings.TrimSpace(vectorColumn)
	if vectorColumn == "" {
		return nil, fmt.Errorf("vector source column is required")
	}

	query := fmt.Sprintf(
		`SELECT rowid, %s FROM %s WHERE %s IS NOT NULL`,
		quoteIdentifier(vectorColumn),
		quoteIdentifier(tableName),
		quoteIdentifier(vectorColumn),
	)
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make(map[string][]float64)
	for rows.Next() {
		var rowID int64
		var raw any
		if err := rows.Scan(&rowID, &raw); err != nil {
			return nil, err
		}
		vectorText := parseVectorRawValue(raw)
		if len(vectorText) == 0 {
			continue
		}
		parsed := make([]float64, 0, len(vectorText))
		parsed = append(parsed, vectorText...)
		candidates[strconv.FormatInt(rowID, 10)] = parsed
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func parseVectorRawValue(raw any) []float64 {
	switch value := raw.(type) {
	case nil:
		return nil
	case []float64:
		if len(value) == 0 {
			return nil
		}
		output := make([]float64, len(value))
		copy(output, value)
		return output
	case []any:
		output := make([]float64, 0, len(value))
		for _, item := range value {
			if typed, ok := item.(float64); ok {
				output = append(output, typed)
			}
		}
		return output
	case string:
		var parsed []float64
		if strings.TrimSpace(value) == "" {
			return nil
		}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil
		}
		return parsed
	case []byte:
		var parsed []float64
		if len(value) == 0 {
			return nil
		}
		if err := json.Unmarshal(value, &parsed); err != nil {
			return nil
		}
		return parsed
	default:
		return nil
	}
}
