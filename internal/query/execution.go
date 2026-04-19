package query

import (
	"database/sql"
	"time"
)

type QueryExecutionResult struct {
	SQL                  string
	Columns              []string
	Rows                 [][]any
	RowCount             int
	DurationMilliseconds int64
}

func ExecuteGraph(database *sql.DB, graph QueryGraph) (*QueryExecutionResult, error) {
	sqlText, err := GenerateSQL(graph)
	if err != nil {
		return nil, err
	}
	return ExecuteSQL(database, sqlText.SQL, sqlText.Parameters...)
}

func ExecuteSQL(database *sql.DB, sqlText string, parameters ...any) (*QueryExecutionResult, error) {
	start := time.Now()
	rows, err := database.Query(sqlText, parameters...)
	if err != nil {
		logExecution(database, sqlText, parameters, 0, time.Since(start).Milliseconds(), err)
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		logExecution(database, sqlText, parameters, 0, time.Since(start).Milliseconds(), err)
		return nil, err
	}

	queryResult := &QueryExecutionResult{
		SQL:     sqlText,
		Columns: append([]string(nil), columns...),
		Rows:    nil,
	}

	for rows.Next() {
		scanTargets := make([]interface{}, len(columns))
		for i := range scanTargets {
			scanTargets[i] = new(interface{})
		}
		if err := rows.Scan(scanTargets...); err != nil {
			logExecution(database, sqlText, parameters, queryResult.RowCount, time.Since(start).Milliseconds(), err)
			return nil, err
		}

		row := make([]any, len(columns))
		for i := range scanTargets {
			row[i] = normalizeQueryValue(*(scanTargets[i].(*interface{})))
		}
		queryResult.Rows = append(queryResult.Rows, row)
		queryResult.RowCount++
	}
	if err := rows.Err(); err != nil {
		logExecution(database, sqlText, parameters, queryResult.RowCount, time.Since(start).Milliseconds(), err)
		return nil, err
	}
	queryResult.DurationMilliseconds = time.Since(start).Milliseconds()
	logExecution(database, sqlText, parameters, queryResult.RowCount, queryResult.DurationMilliseconds, nil)
	return queryResult, nil
}

func logExecution(database *sql.DB, sqlText string, parameters []any, rowCount int, duration int64, executionError error) {
	history := NewQueryHistoryRepository(database)
	_ = history.Log(sqlText, parameters, rowCount, duration, executionError)
}

func normalizeQueryValue(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return v
	}
}
