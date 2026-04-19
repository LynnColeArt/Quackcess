package terminal

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LynnColeArt/Quackcess/internal/canvasservice"
	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/query"
	"github.com/LynnColeArt/Quackcess/internal/vector"
)

const (
	TerminalKindQuery   = "query"
	TerminalKindHistory = "history"
	TerminalKindHelp    = "help"
	TerminalKindError   = "error"
)

type VectorService interface {
	ListVectorFields() ([]vector.VectorField, error)
	RebuildVector(fieldID string, force bool) (vector.VectorBuildResult, error)
	SearchVector(fieldID string, queryText string, limit int) (vector.VectorSearchResult, error)
}

type vectorRebuildWithFilter interface {
	RebuildVectorWithFilter(fieldID string, filter string, force bool) (vector.VectorBuildResult, error)
}

type TerminalResult struct {
	Kind                 string
	Input                string
	SQLText              string
	Parameters           []any
	Columns              []string
	Rows                 [][]any
	RowCount             int
	DurationMilliseconds int64
	ErrorText            string
	Message              string
	History              []query.QueryHistory
	Vectorize            *TerminalVectorizeMetadata
}

type TerminalVectorizeMetadata struct {
	TableName    string
	SourceColumn string
	TargetColumn string
	Filter       string
	FieldID      string
	Built        bool
	BatchSize    int
	VectorCount  int
	SkipReason   string
}

type TerminalService struct {
	db               *sql.DB
	history          *query.QueryHistoryRepository
	maxItems         int
	console          *EventConsole
	canvasRepository *catalog.CanvasRepository
	canvasService    *canvasservice.CanvasArtifactService
	vectorService    VectorService
}

const canvasExecutionDefaultLimit = 200

func NewTerminalService(database *sql.DB) *TerminalService {
	return NewTerminalServiceWithCanvasRepository(database, nil, nil)
}

func NewTerminalServiceWithConsole(database *sql.DB, console *EventConsole) *TerminalService {
	return NewTerminalServiceWithCanvasRepository(database, console, nil)
}

func NewTerminalServiceWithCanvasRepository(database *sql.DB, console *EventConsole, canvasRepository *catalog.CanvasRepository) *TerminalService {
	return NewTerminalServiceWithCanvasRepositoryAndVectorService(database, console, canvasRepository, nil)
}

func NewTerminalServiceWithCanvasRepositoryAndVectorService(
	database *sql.DB,
	console *EventConsole,
	canvasRepository *catalog.CanvasRepository,
	vectorService VectorService,
) *TerminalService {
	var canvasService *canvasservice.CanvasArtifactService
	if canvasRepository != nil {
		canvasService = canvasservice.NewCanvasArtifactService(canvasRepository)
	}
	return &TerminalService{
		db:               database,
		history:          query.NewQueryHistoryRepository(database),
		console:          console,
		maxItems:         50,
		canvasRepository: canvasRepository,
		canvasService:    canvasService,
		vectorService:    vectorService,
	}
}

func (s *TerminalService) RunCommand(input string) (TerminalResult, error) {
	if s == nil || s.db == nil {
		return TerminalResult{}, fmt.Errorf("terminal is not initialized")
	}
	return s.runCommand(strings.TrimSpace(input))
}

func (s *TerminalService) runCommand(input string) (TerminalResult, error) {
	if input == "" {
		return s.helpResult(input), nil
	}

	if strings.HasPrefix(input, "\\") {
		return s.handleBackslashCommand(input)
	}

	if strings.EqualFold(input, "help") {
		return s.helpResult(input), nil
	}

	if isVectorizeSyntax(input) {
		return s.vectorizeResult(input)
	}

	return s.executeQuery(input)
}

func (s *TerminalService) executeQuery(sqlText string) (TerminalResult, error) {
	queryResult, err := query.ExecuteSQL(s.db, sqlText)
	if err != nil {
		s.logConsoleEvent("query.failed", "terminal", err.Error())
		return TerminalResult{
			Kind:                 TerminalKindError,
			Input:                sqlText,
			ErrorText:            err.Error(),
			SQLText:              sqlText,
			RowCount:             0,
			DurationMilliseconds: 0,
		}, nil
	}

	s.logConsoleEvent("query.executed", "terminal", queryResult.SQL)

	return TerminalResult{
		Kind:                 TerminalKindQuery,
		Input:                sqlText,
		SQLText:              queryResult.SQL,
		Columns:              queryResult.Columns,
		Rows:                 queryResult.Rows,
		RowCount:             queryResult.RowCount,
		DurationMilliseconds: queryResult.DurationMilliseconds,
	}, nil
}

func (s *TerminalService) handleBackslashCommand(input string) (TerminalResult, error) {
	fields := strings.Fields(input)
	switch strings.ToLower(fields[0]) {
	case "\\help", "\\h":
		return s.helpResult(input), nil
	case "\\history":
		limit, err := parseHistoryLimit(fields)
		if err != nil {
			return TerminalResult{
				Kind:      TerminalKindError,
				Input:     input,
				ErrorText: err.Error(),
			}, nil
		}
		return s.historyResult(limit)
	case "\\canvas":
		if len(fields) < 2 {
			return TerminalResult{
				Kind:      TerminalKindError,
				Input:     input,
				ErrorText: "canvas usage: \\canvas <name> | \\canvas new <name> | \\canvas rename <old-name> <new-name> | \\canvas delete <name> | \\canvas save <name> <spec-json>",
			}, nil
		}
		switch strings.ToLower(fields[1]) {
		case "new":
			name, err := parseCanvasSimpleName(input, "\\canvas new")
			if err != nil {
				return TerminalResult{
					Kind:      TerminalKindError,
					Input:     input,
					ErrorText: err.Error(),
				}, nil
			}
			return s.canvasNewResult(input, name)
		case "rename":
			names, err := parseCanvasRenameNames(input)
			if err != nil {
				return TerminalResult{
					Kind:      TerminalKindError,
					Input:     input,
					ErrorText: err.Error(),
				}, nil
			}
			return s.canvasRenameResult(input, names[0], names[1])
		case "delete":
			name, err := parseCanvasSimpleName(input, "\\canvas delete")
			if err != nil {
				return TerminalResult{
					Kind:      TerminalKindError,
					Input:     input,
					ErrorText: err.Error(),
				}, nil
			}
			return s.canvasDeleteResult(input, name)
		case "save":
			payload, err := parseCanvasSavePayload(input)
			if err != nil {
				return TerminalResult{
					Kind:      TerminalKindError,
					Input:     input,
					ErrorText: err.Error(),
				}, nil
			}
			return s.canvasSaveResult(input, payload.name, payload.specJSON)
		default:
			name, err := parseCanvasName(fields, input)
			if err != nil {
				return TerminalResult{
					Kind:      TerminalKindError,
					Input:     input,
					ErrorText: err.Error(),
				}, nil
			}
			return s.canvasResult(input, name)
		}
	case "\\vector":
		return s.vectorResult(input, fields)
	default:
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "unknown command",
		}, nil
	}
}

func (s *TerminalService) historyResult(limit int) (TerminalResult, error) {
	if limit > s.maxItems {
		limit = s.maxItems
	}
	entries, err := s.history.ListRecent(limit)
	if err != nil {
		return TerminalResult{}, err
	}
	return TerminalResult{
		Kind:                 TerminalKindHistory,
		Input:                "\\history",
		History:              entries,
		RowCount:             len(entries),
		DurationMilliseconds: 0,
		Message:              buildHistoryMessage(entries),
	}, nil
}

func (s *TerminalService) helpResult(input string) TerminalResult {
	return TerminalResult{
		Kind:    TerminalKindHelp,
		Input:   input,
		Message: "type SQL directly, or \\history [n], \\canvas <name>, \\canvas new <name>, \\canvas rename <old-name> <new-name>, \\canvas delete <name>, \\canvas save <name> <spec-json>, \\vector list, \\vector rebuild <field-id> [--force], \\vector search <field-id> <query> [--limit <n>], UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>] and \\help / \\h",
	}
}

func isVectorizeSyntax(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}
	fields := strings.Fields(trimmed)
	return len(fields) >= 4 &&
		strings.EqualFold(fields[0], "update") &&
		strings.EqualFold(fields[2], "vectorize")
}

func (s *TerminalService) vectorizeResult(input string) (TerminalResult, error) {
	tableName, sourceColumn, targetColumn, filter, err := parseUpdateVectorize(input)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}
	if s.vectorService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "vector service is not configured",
		}, nil
	}

	field, err := s.resolveVectorFieldForTableAndSource(tableName, sourceColumn, targetColumn)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}

	result, err := s.rebuildVectorWithFilter(field.ID, filter)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}

	status := "skipped"
	if result.Built {
		status = "built"
	}
	columns := []string{"field_id", "status", "batch_size", "vector_count", "skip_reason", "table_name", "source_column", "target_column", "where"}
	row := []any{
		result.Field.ID,
		status,
		result.BatchSize,
		len(result.VectorsByID),
		result.SkipReason,
		result.Field.TableName,
		result.Field.SourceColumn,
		result.Field.VectorColumn,
		filter,
	}
	s.logConsoleEvent("vector.rebuild", "terminal", input)
	metadata := &TerminalVectorizeMetadata{
		TableName:    result.Field.TableName,
		SourceColumn: result.Field.SourceColumn,
		TargetColumn: result.Field.VectorColumn,
		Filter:       filter,
		FieldID:      result.Field.ID,
		Built:        result.Built,
		BatchSize:    result.BatchSize,
		VectorCount:  len(result.VectorsByID),
		SkipReason:   result.SkipReason,
	}

	return TerminalResult{
		Kind:                 TerminalKindQuery,
		Input:                input,
		SQLText:              "vectorize",
		Columns:              columns,
		Rows:                 [][]any{row},
		RowCount:             1,
		DurationMilliseconds: 0,
		Message:              "vectorize completed",
		Vectorize:            metadata,
	}, nil
}

func (s *TerminalService) rebuildVectorWithFilter(fieldID, filter string) (vector.VectorBuildResult, error) {
	if filter == "" {
		return s.vectorService.RebuildVector(fieldID, true)
	}
	if filtered, ok := s.vectorService.(vectorRebuildWithFilter); ok {
		return filtered.RebuildVectorWithFilter(fieldID, filter, true)
	}
	return vector.VectorBuildResult{}, fmt.Errorf("vector service does not support filtered vectorize commands")
}

func (s *TerminalService) resolveVectorFieldForTableAndSource(
	tableName, sourceColumn, targetColumn string,
) (vector.VectorField, error) {
	fields, err := s.vectorService.ListVectorFields()
	if err != nil {
		return vector.VectorField{}, err
	}
	matches := make([]vector.VectorField, 0)
	for _, raw := range fields {
		canonical, err := vector.CanonicalizeVectorField(raw)
		if err != nil {
			return vector.VectorField{}, err
		}
		if normalizeIdentifierMatch(canonical.TableName) == normalizeIdentifierMatch(tableName) &&
			normalizeIdentifierMatch(canonical.SourceColumn) == normalizeIdentifierMatch(sourceColumn) {
			if targetColumn == "" {
				matches = append(matches, canonical)
				continue
			}
			if normalizeIdentifierMatch(canonical.VectorColumn) == normalizeIdentifierMatch(targetColumn) {
				return canonical, nil
			}
		}
	}

	if targetColumn == "" {
		if len(matches) == 0 {
			return vector.VectorField{}, fmt.Errorf(
				"vector field not found for %s.%s",
				tableName,
				sourceColumn,
			)
		}
		if len(matches) == 1 {
			return matches[0], nil
		}
		return vector.VectorField{}, fmt.Errorf(
			"multiple vector fields found for %s.%s; include target vector column",
			tableName,
			sourceColumn,
		)
	}
	return vector.VectorField{}, fmt.Errorf("vector field not found for %s.%s AS %s", tableName, sourceColumn, targetColumn)
}

func parseUpdateVectorize(input string) (string, string, string, string, error) {
	const usage = "vectorize usage: UPDATE <table> VECTORIZE <source-column> [AS] <target-vector-column> [WHERE <filter>]"
	trimmed := strings.TrimSpace(input)
	fields := strings.Fields(trimmed)
	if len(fields) < 4 {
		return "", "", "", "", fmt.Errorf(usage)
	}
	if !strings.EqualFold(fields[0], "update") {
		return "", "", "", "", fmt.Errorf(usage)
	}
	if !strings.EqualFold(fields[2], "vectorize") {
		return "", "", "", "", fmt.Errorf(usage)
	}
	tableName := fields[1]
	sourceColumn := fields[3]
	targetColumn := ""
	cursor := 4
	if cursor >= len(fields) {
		return tableName, sourceColumn, "", "", nil
	}

	firstTail := strings.ToLower(fields[cursor])
	if firstTail == "where" {
		filter := strings.Join(fields[cursor+1:], " ")
		if filter == "" {
			return "", "", "", "", fmt.Errorf(usage)
		}
		return tableName, sourceColumn, "", filter, nil
	}

	if strings.EqualFold(fields[4], "as") {
		if len(fields) < 6 {
			return "", "", "", "", fmt.Errorf(usage)
		}
		targetColumn = fields[5]
		cursor = 6
	} else {
		targetColumn = fields[4]
		cursor = 5
	}
	if tableName == "" || sourceColumn == "" || targetColumn == "" {
		return "", "", "", "", fmt.Errorf(usage)
	}

	if cursor >= len(fields) {
		return tableName, sourceColumn, targetColumn, "", nil
	}

	if !strings.EqualFold(fields[cursor], "where") {
		return "", "", "", "", fmt.Errorf(usage)
	}

	filter := strings.Join(fields[cursor+1:], " ")
	if filter == "" {
		return "", "", "", "", fmt.Errorf(usage)
	}
	return tableName, sourceColumn, targetColumn, filter, nil
}

func normalizeIdentifierMatch(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`+"`"))
}

func parseCanvasName(fields []string, input string) (string, error) {
	if len(fields) < 2 {
		return "", fmt.Errorf("canvas usage: \\canvas <name>")
	}
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), fields[0]))
	if name == "" {
		return "", fmt.Errorf("canvas name is required")
	}
	return name, nil
}

type canvasSavePayload struct {
	name     string
	specJSON string
}

func parseCanvasSavePayload(input string) (canvasSavePayload, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\canvas save"))
	if raw == "" {
		return canvasSavePayload{}, fmt.Errorf("canvas usage: \\canvas save <name> <spec-json>")
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 {
		return canvasSavePayload{}, fmt.Errorf("canvas usage: \\canvas save <name> <spec-json>")
	}
	name := strings.TrimSpace(parts[0])
	specJSON := strings.TrimSpace(parts[1])
	if name == "" || specJSON == "" {
		return canvasSavePayload{}, fmt.Errorf("canvas usage: \\canvas save <name> <spec-json>")
	}
	if len(specJSON) >= 2 && ((specJSON[0] == '\'' && specJSON[len(specJSON)-1] == '\'') || (specJSON[0] == '"' && specJSON[len(specJSON)-1] == '"')) {
		specJSON = strings.Trim(specJSON, string(specJSON[0]))
	}
	return canvasSavePayload{name: name, specJSON: specJSON}, nil
}

func parseCanvasRenameNames(input string) ([2]string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\canvas rename"))
	if raw == "" {
		return [2]string{}, fmt.Errorf("canvas usage: \\canvas rename <old-name> <new-name>")
	}
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return [2]string{}, fmt.Errorf("canvas usage: \\canvas rename <old-name> <new-name>")
	}
	if parts[0] == "" || parts[1] == "" {
		return [2]string{}, fmt.Errorf("canvas usage: \\canvas rename <old-name> <new-name>")
	}
	return [2]string{parts[0], parts[1]}, nil
}

func parseCanvasSimpleName(input, commandPrefix string) (string, error) {
	name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), commandPrefix))
	if name == "" {
		return "", fmt.Errorf("canvas usage: %s <name>", commandPrefix)
	}
	return name, nil
}

func (s *TerminalService) canvasNewResult(input, name string) (TerminalResult, error) {
	if s.canvasService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "canvas service is not configured",
		}, nil
	}
	canvas, err := s.canvasService.CreateDraftCanvas(name)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}
	return TerminalResult{
		Kind:    TerminalKindHelp,
		Input:   input,
		Message: "created canvas " + canvas.Name,
	}, nil
}

func (s *TerminalService) canvasRenameResult(input, oldName, newName string) (TerminalResult, error) {
	if s.canvasService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "canvas service is not configured",
		}, nil
	}
	if err := s.canvasService.RenameCanvas(oldName, newName); err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}
	return TerminalResult{
		Kind:    TerminalKindHelp,
		Input:   input,
		Message: "renamed canvas " + oldName + " -> " + newName,
	}, nil
}

func (s *TerminalService) canvasDeleteResult(input, name string) (TerminalResult, error) {
	if s.canvasService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "canvas service is not configured",
		}, nil
	}
	if err := s.canvasService.DeleteCanvas(name); err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}
	return TerminalResult{
		Kind:    TerminalKindHelp,
		Input:   input,
		Message: "deleted canvas " + name,
	}, nil
}

func (s *TerminalService) canvasSaveResult(input, name, specJSON string) (TerminalResult, error) {
	if s.canvasService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "canvas service is not configured",
		}, nil
	}
	if err := s.canvasService.SaveCanvasSpec(name, specJSON, "terminal"); err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}
	return TerminalResult{
		Kind:    TerminalKindHelp,
		Input:   input,
		Message: "saved canvas " + name,
	}, nil
}

func (s *TerminalService) canvasResult(input string, name string) (TerminalResult, error) {
	if s.canvasService == nil {
		return TerminalResult{
			Kind:                 TerminalKindError,
			Input:                input,
			ErrorText:            "canvas repository is not configured",
			DurationMilliseconds: 0,
		}, nil
	}

	spec, err := s.canvasService.GetForExecution(name)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}

	sqlText, err := query.GenerateSQLFromCanvasWithLimit(spec, canvasExecutionDefaultLimit)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}

	queryResult, err := query.ExecuteSQL(s.db, sqlText.SQL, sqlText.Parameters...)
	if err != nil {
		s.logConsoleEvent("query.failed", "terminal", err.Error())
		return TerminalResult{
			Kind:                 TerminalKindError,
			Input:                input,
			ErrorText:            err.Error(),
			SQLText:              sqlText.SQL,
			Parameters:           append([]any(nil), sqlText.Parameters...),
			DurationMilliseconds: 0,
		}, nil
	}

	s.logConsoleEvent("query.executed", "terminal", sqlText.SQL)

	return TerminalResult{
		Kind:                 TerminalKindQuery,
		Input:                input,
		SQLText:              sqlText.SQL,
		Parameters:           append([]any(nil), sqlText.Parameters...),
		Columns:              queryResult.Columns,
		Rows:                 queryResult.Rows,
		RowCount:             queryResult.RowCount,
		DurationMilliseconds: queryResult.DurationMilliseconds,
	}, nil
}

func parseHistoryLimit(fields []string) (int, error) {
	if len(fields) == 1 {
		return 25, nil
	}
	if len(fields) != 2 {
		return 0, fmt.Errorf("history usage: \\history [n]")
	}
	limit, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("history usage: \\history [n]")
	}
	if limit <= 0 {
		return 0, fmt.Errorf("history usage: \\history [n]")
	}
	return limit, nil
}

func buildHistoryMessage(entries []query.QueryHistory) string {
	var lines []string
	for _, entry := range entries {
		line := entry.ExecutedAt.Format("2006-01-02T15:04:05")
		if entry.Success {
			line += " [OK] "
		} else {
			line += " [ERR] "
		}
		line += entry.SQLText
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (s *TerminalService) logConsoleEvent(kind, source, message string) {
	if s == nil || s.console == nil {
		return
	}
	s.console.Append(kind, source, message)
}

func (s *TerminalService) vectorResult(input string, fields []string) (TerminalResult, error) {
	if len(fields) < 2 {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "vector usage: \\vector list | \\vector rebuild <field-id> [--force] | \\vector search <field-id> <query> [--limit <n>]",
		}, nil
	}
	command := strings.ToLower(fields[1])
	switch command {
	case "list":
		if len(fields) != 2 {
			return TerminalResult{
				Kind:      TerminalKindError,
				Input:     input,
				ErrorText: "vector usage: \\vector list",
			}, nil
		}
		return s.vectorListResult(input)
	case "rebuild":
		id, force, err := parseVectorRebuild(fields, input)
		if err != nil {
			return TerminalResult{
				Kind:      TerminalKindError,
				Input:     input,
				ErrorText: err.Error(),
			}, nil
		}
		return s.vectorRebuildResult(input, id, force)
	case "search":
		fieldID, queryText, limit, err := parseVectorSearch(fields, input)
		if err != nil {
			return TerminalResult{
				Kind:      TerminalKindError,
				Input:     input,
				ErrorText: err.Error(),
			}, nil
		}
		return s.vectorSearchResult(input, fieldID, queryText, limit)
	default:
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "vector usage: \\vector list | \\vector rebuild <field-id> [--force] | \\vector search <field-id> <query> [--limit <n>]",
		}, nil
	}
}

func (s *TerminalService) vectorListResult(input string) (TerminalResult, error) {
	if s.vectorService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "vector service is not configured",
		}, nil
	}

	fields, err := s.vectorService.ListVectorFields()
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}

	rows := make([][]any, 0, len(fields))
	for _, field := range fields {
		normalized, err := vector.CanonicalizeVectorField(field)
		if err != nil {
			return TerminalResult{
				Kind:      TerminalKindError,
				Input:     input,
				ErrorText: err.Error(),
			}, nil
		}
		rows = append(rows, []any{
			normalized.ID,
			normalized.TableName,
			normalized.SourceColumn,
			normalized.VectorColumn,
			normalized.Provider,
			normalized.Model,
			normalized.Dimension,
			normalized.StaleAfterHours,
			formatVectorFieldTime(normalized.LastIndexedAt),
			formatVectorFieldTime(normalized.SourceLastUpdatedAt),
		})
	}

	columns := []string{"id", "table_name", "source_column", "vector_column", "provider", "model", "dimension", "stale_after_hours", "last_indexed_at", "source_last_updated_at"}
	return TerminalResult{
		Kind:                 TerminalKindQuery,
		Input:                input,
		SQLText:              "vector list",
		Columns:              columns,
		Rows:                 rows,
		RowCount:             len(rows),
		DurationMilliseconds: 0,
		Message:              "vector fields listed",
	}, nil
}

func (s *TerminalService) vectorRebuildResult(input, fieldID string, force bool) (TerminalResult, error) {
	if s.vectorService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "vector service is not configured",
		}, nil
	}

	result, err := s.vectorService.RebuildVector(fieldID, force)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}
	status := "skipped"
	if result.Built {
		status = "built"
	}
	columns := []string{"field_id", "status", "batch_size", "vector_count", "skip_reason", "table_name", "source_column", "stale_after_hours", "last_indexed_at"}
	field := result.Field
	row := []any{
		field.ID,
		status,
		result.BatchSize,
		len(result.VectorsByID),
		result.SkipReason,
		field.TableName,
		field.SourceColumn,
		field.StaleAfterHours,
		formatVectorFieldTime(field.LastIndexedAt),
	}
	s.logConsoleEvent("vector.rebuild", "terminal", input)
	return TerminalResult{
		Kind:                 TerminalKindQuery,
		Input:                input,
		SQLText:              "vector rebuild",
		Columns:              columns,
		Rows:                 [][]any{row},
		RowCount:             1,
		DurationMilliseconds: 0,
		Message:              "vector rebuild completed",
	}, nil
}

func (s *TerminalService) vectorSearchResult(input, fieldID, queryText string, limit int) (TerminalResult, error) {
	if s.vectorService == nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: "vector service is not configured",
		}, nil
	}

	result, err := s.vectorService.SearchVector(fieldID, queryText, limit)
	if err != nil {
		return TerminalResult{
			Kind:      TerminalKindError,
			Input:     input,
			ErrorText: err.Error(),
		}, nil
	}

	rows := make([][]any, 0, len(result.Matches))
	for _, match := range result.Matches {
		rows = append(rows, []any{match.ID, match.Score})
	}

	return TerminalResult{
		Kind:                 TerminalKindQuery,
		Input:                input,
		SQLText:              "vector search",
		Columns:              []string{"id", "score"},
		Rows:                 rows,
		RowCount:             len(rows),
		DurationMilliseconds: 0,
		Message:              "vector search completed",
	}, nil
}

func parseVectorRebuild(fields []string, input string) (string, bool, error) {
	if len(fields) < 3 || len(fields) > 4 {
		return "", false, fmt.Errorf("vector usage: \\vector rebuild <field-id> [--force]")
	}

	if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\vector rebuild")) == "" {
		return "", false, fmt.Errorf("vector usage: \\vector rebuild <field-id> [--force]")
	}

	id := strings.TrimSpace(fields[2])
	force := false
	if len(fields) == 4 {
		if strings.TrimSpace(fields[3]) == "--force" {
			force = true
		} else {
			return "", false, fmt.Errorf("vector usage: \\vector rebuild <field-id> [--force]")
		}
	}
	if id == "" {
		return "", false, fmt.Errorf("vector usage: \\vector rebuild <field-id> [--force]")
	}
	return id, force, nil
}

func parseVectorSearch(fields []string, input string) (string, string, int, error) {
	if len(fields) < 3 {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}

	if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\vector search")) == "" {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}

	remainder := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(input), "\\vector search"))
	parts := strings.Fields(remainder)
	if len(parts) < 2 {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}

	fieldID := strings.TrimSpace(parts[0])
	if fieldID == "" {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}
	queryTokens := strings.Fields(strings.TrimSpace(strings.TrimPrefix(remainder, fieldID)))
	if len(queryTokens) == 0 {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}
	if queryTokens[0] == "--limit" {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}
	if queryTokens[len(queryTokens)-1] == "--limit" {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}

	limit := 10
	if len(queryTokens) > 2 && queryTokens[len(queryTokens)-2] == "--limit" {
		parsed, err := strconv.Atoi(queryTokens[len(queryTokens)-1])
		if err != nil || parsed <= 0 {
			return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
		}
		limit = parsed
		queryTokens = queryTokens[:len(queryTokens)-2]
		if len(queryTokens) == 0 {
			return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
		}
	}

	queryText := strings.TrimSpace(strings.Join(queryTokens, " "))
	if queryText == "" {
		return "", "", 0, fmt.Errorf("vector usage: \\vector search <field-id> <query> [--limit <n>]")
	}
	return fieldID, queryText, limit, nil
}

func formatVectorFieldTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
