package query

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type QueryHistory struct {
	ExecutedAt           time.Time
	SQLText              string
	ParametersJSON       string
	RowCount             int
	DurationMilliseconds int64
	Success              bool
	ErrorText            string
}

type QueryHistoryRepository struct {
	db *sql.DB
}

func NewQueryHistoryRepository(sqlDB *sql.DB) *QueryHistoryRepository {
	return &QueryHistoryRepository{db: sqlDB}
}

func (r *QueryHistoryRepository) Log(
	sqlText string,
	parameters []any,
	rowCount int,
	durationMilliseconds int64,
	executionError error,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("query history repository is not initialized")
	}
	if sqlText == "" {
		return fmt.Errorf("sql text is required")
	}

	parametersJSON, err := marshalParameters(parameters)
	if err != nil {
		return err
	}

	success := executionError == nil
	errorText := ""
	if executionError != nil {
		errorText = executionError.Error()
	}

	const statement = `
		INSERT INTO quackcess_query_history
			(executed_at, sql_text, parameters_json, row_count, duration_milliseconds, success, error_text)
		VALUES (CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?);
	`
	_, err = r.db.Exec(statement, sqlText, parametersJSON, rowCount, durationMilliseconds, success, nullString(errorText))
	return err
}

func (r *QueryHistoryRepository) ListRecent(limit int) ([]QueryHistory, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("query history repository is not initialized")
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Query(
		`
		SELECT executed_at, sql_text, parameters_json, row_count, duration_milliseconds, success, error_text
		FROM quackcess_query_history
		ORDER BY executed_at DESC
		LIMIT ?;
	`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []QueryHistory
	for rows.Next() {
		var item QueryHistory
		var parametersJSON sql.NullString
		var errorText sql.NullString
		if err := rows.Scan(&item.ExecutedAt, &item.SQLText, &parametersJSON, &item.RowCount, &item.DurationMilliseconds, &item.Success, &errorText); err != nil {
			return nil, err
		}
		item.ParametersJSON = parametersJSON.String
		item.ErrorText = errorText.String
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return history, nil
}

func marshalParameters(parameters []any) (string, error) {
	if len(parameters) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(parameters)
	if err != nil {
		return "", fmt.Errorf("marshal parameters: %w", err)
	}
	return string(raw), nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
