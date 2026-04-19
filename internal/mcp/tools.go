package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/LynnColeArt/Quackcess/internal/report"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
	"github.com/LynnColeArt/Quackcess/internal/vector"
)

type QueryRunner interface {
	RunCommand(input string) (terminal.TerminalResult, error)
}

type CatalogService interface {
	ListTables() ([]string, error)
	ListViews() ([]string, error)
	ListCanvases() ([]string, error)
}

type ArtifactStore interface {
	Upsert(id string, payload string) error
	Get(id string) (string, error)
	Delete(id string) error
	List() ([]string, error)
}

type ProjectReportService interface {
	ListReportIDs() ([]string, error)
	ListChartIDs() ([]string, error)
	LoadReportExport(reportID string, chartData map[string][]report.ExportRow) (report.ReportExport, error)
}

type VectorService interface {
	ListVectorFields() ([]vector.VectorField, error)
	RebuildVector(fieldID string, force bool) (vector.VectorBuildResult, error)
	SearchVector(fieldID, queryText string, limit int) (vector.VectorSearchResult, error)
}

type CoreTools struct {
	QueryRunner    QueryRunner
	CatalogService CatalogService
	Artifacts      ArtifactStore
	Reports        ProjectReportService
	Vector         VectorService
	EventBus       *EventBus
}

type vectorRebuildServiceWithProgress interface {
	RebuildVectorWithProgress(fieldID string, force bool, callbacks ...vector.VectorBuildProgressHandler) (vector.VectorBuildResult, error)
}

func RegisterCoreTools(server *Server, core CoreTools) error {
	if server == nil {
		return fmt.Errorf("server is nil")
	}
	if err := server.RegisterTool(ToolDefinition{
		Name:        "system.ping",
		Description: "health check for MCP transport",
		Handler:     handleSystemPing,
	}); err != nil {
		return err
	}

	if core.QueryRunner != nil {
		if err := server.RegisterTool(ToolDefinition{
			Name:        "query.execute",
			Description: "execute SQL against DuckDB-backed project context",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleQueryExecute(core.QueryRunner, args)
			},
		}); err != nil {
			return err
		}
	}

	if core.CatalogService != nil {
		if err := server.RegisterTool(ToolDefinition{
			Name:        "schema.inspect",
			Description: "list tables, views, and canvases in the current project",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleSchemaInspect(core.CatalogService, args)
			},
		}); err != nil {
			return err
		}
	}

	if core.Artifacts != nil {
		if err := server.RegisterTool(ToolDefinition{
			Name:        "artifact.set",
			Description: "insert or replace an artifact payload by id",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleArtifactSet(core.Artifacts, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "artifact.get",
			Description: "read an artifact payload by id",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleArtifactGet(core.Artifacts, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "artifact.delete",
			Description: "delete an artifact payload by id",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleArtifactDelete(core.Artifacts, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "artifact.list",
			Description: "list artifact ids",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleArtifactList(core.Artifacts, args)
			},
		}); err != nil {
			return err
		}
	}
	if core.Reports != nil {
		if err := server.RegisterTool(ToolDefinition{
			Name:        "chart.list",
			Description: "list chart artifact ids in the active project",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleReportChartList(core.Reports, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "report.list",
			Description: "list report artifact ids in the active project",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleReportList(core.Reports, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "report.export",
			Description: "render and export a report with CSV/image placeholders",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleReportExport(core.Reports, args)
			},
		}); err != nil {
			return err
		}
	}
	if core.Vector != nil {
		if err := server.RegisterTool(ToolDefinition{
			Name:        "vector.list",
			Description: "list configured vector fields in the active project",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleVectorList(core.Vector, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "vector.rebuild",
			Description: "build or rebuild a configured vector field embedding index",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleVectorRebuild(core.Vector, core.EventBus, principal, args)
			},
		}); err != nil {
			return err
		}
		if err := server.RegisterTool(ToolDefinition{
			Name:        "vector.search",
			Description: "execute semantic vector search against a configured vector field",
			Handler: func(principal string, args json.RawMessage) (any, *ToolError) {
				return handleVectorSearch(core.Vector, args)
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func handleSystemPing(principal string, args json.RawMessage) (any, *ToolError) {
	return map[string]any{
		"pong":      true,
		"principal": principal,
	}, nil
}

func handleQueryExecute(runner QueryRunner, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		SQL string `json:"sql"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	if strings.TrimSpace(args.SQL) == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "sql is required")
	}

	result, err := runner.RunCommand(args.SQL)
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}

	return map[string]any{
		"kind":     result.Kind,
		"sql":      result.SQLText,
		"columns":  result.Columns,
		"rows":     normalizeToolRows(result.Rows),
		"rowCount": result.RowCount,
		"duration": result.DurationMilliseconds,
		"error":    result.ErrorText,
	}, nil
}

func handleSchemaInspect(service CatalogService, rawArgs json.RawMessage) (any, *ToolError) {
	if err := parseArgs(rawArgs, &struct{}{}); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	tables, err := service.ListTables()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	views, err := service.ListViews()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	canvases, err := service.ListCanvases()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	sort.Strings(tables)
	sort.Strings(views)
	sort.Strings(canvases)
	return map[string]any{
		"tables":   tables,
		"views":    views,
		"canvases": canvases,
	}, nil
}

func handleArtifactSet(store ArtifactStore, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		ID      string `json:"id"`
		Payload string `json:"payload"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	id := strings.TrimSpace(args.ID)
	if id == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "id is required")
	}
	if err := store.Upsert(id, args.Payload); err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{"id": id, "status": "updated"}, nil
}

func handleArtifactGet(store ArtifactStore, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		ID string `json:"id"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	id := strings.TrimSpace(args.ID)
	if id == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "id is required")
	}
	payload, err := store.Get(id)
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{"id": id, "payload": payload}, nil
}

func handleArtifactDelete(store ArtifactStore, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		ID string `json:"id"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	id := strings.TrimSpace(args.ID)
	if id == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "id is required")
	}
	if err := store.Delete(id); err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{"id": id, "status": "deleted"}, nil
}

func handleArtifactList(store ArtifactStore, rawArgs json.RawMessage) (any, *ToolError) {
	if err := parseArgs(rawArgs, &struct{}{}); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	items, err := store.List()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{"items": items}, nil
}

func handleReportList(service ProjectReportService, rawArgs json.RawMessage) (any, *ToolError) {
	if err := parseArgs(rawArgs, &struct{}{}); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	reports, err := service.ListReportIDs()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{"items": reports}, nil
}

func handleReportChartList(service ProjectReportService, rawArgs json.RawMessage) (any, *ToolError) {
	if err := parseArgs(rawArgs, &struct{}{}); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	charts, err := service.ListChartIDs()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{"items": charts}, nil
}

func handleReportExport(service ProjectReportService, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		ReportID  string                      `json:"reportId"`
		ChartData map[string][]map[string]any `json:"chartData"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	reportID := strings.TrimSpace(args.ReportID)
	if reportID == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "reportId is required")
	}
	normalized := map[string][]report.ExportRow{}
	for chartID, rows := range args.ChartData {
		normalizedRows := make([]report.ExportRow, 0, len(rows))
		for _, row := range rows {
			normalizedRows = append(normalizedRows, report.ExportRow(row))
		}
		normalized[chartID] = normalizedRows
	}
	exported, err := service.LoadReportExport(reportID, normalized)
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{
		"reportId": exported.ReportID,
		"title":    exported.Title,
		"sections": exported.Sections,
	}, nil
}

func handleVectorList(service VectorService, rawArgs json.RawMessage) (any, *ToolError) {
	if err := parseArgs(rawArgs, &struct{}{}); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	fields, err := service.ListVectorFields()
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	items := make([]vector.VectorField, 0, len(fields))
	for _, field := range fields {
		normalized, canonicalizationErr := vector.CanonicalizeVectorField(field)
		if canonicalizationErr != nil {
			return nil, NewToolError(ErrorCodeHandlerError, canonicalizationErr.Error())
		}
		items = append(items, normalized)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return map[string]any{
		"items": items,
	}, nil
}

func handleVectorRebuild(service VectorService, eventBus *EventBus, principal string, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		FieldID string `json:"fieldId"`
		Force   bool   `json:"force"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	fieldID := strings.TrimSpace(args.FieldID)
	if fieldID == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "fieldId is required")
	}

	result, err := rebuildVectorWithProgress(service, eventBus, principal, fieldID, args.Force)
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{
		"fieldId":     result.Field.ID,
		"built":       result.Built,
		"batchSize":   result.BatchSize,
		"vectorCount": len(result.VectorsByID),
		"skipReason":  result.SkipReason,
		"field":       result.Field,
	}, nil
}

func handleVectorSearch(service VectorService, rawArgs json.RawMessage) (any, *ToolError) {
	var args struct {
		FieldID  string `json:"fieldId"`
		Query    string `json:"query"`
		Limit    int    `json:"limit"`
	}
	if err := parseArgs(rawArgs, &args); err != nil {
		return nil, NewToolError(ErrorCodeInvalidArgument, err.Error())
	}
	args.FieldID = strings.TrimSpace(args.FieldID)
	if args.FieldID == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "fieldId is required")
	}
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return nil, NewToolError(ErrorCodeInvalidArgument, "query is required")
	}

	result, err := service.SearchVector(args.FieldID, args.Query, args.Limit)
	if err != nil {
		return nil, NewToolError(ErrorCodeHandlerError, err.Error())
	}
	return map[string]any{
		"fieldId":      result.Field.ID,
		"query":        args.Query,
		"vectorColumn": result.Field.VectorColumn,
		"vectorCount":  len(result.Matches),
		"items":        result.Matches,
	}, nil
}

func rebuildVectorWithProgress(service VectorService, eventBus *EventBus, principal, fieldID string, force bool) (vector.VectorBuildResult, error) {
	if service == nil {
		return vector.VectorBuildResult{}, fmt.Errorf("vector service is nil")
	}
	if handler, ok := service.(vectorRebuildServiceWithProgress); ok {
		return handler.RebuildVectorWithProgress(fieldID, force, func(progress vector.VectorBuildProgress) {
			if eventBus == nil {
				return
			}
			if progress.FieldID == "" {
				progress.FieldID = fieldID
			}
			eventBus.Publish(Event{
				Type:      eventVectorRebuildProgress,
				Tool:      "vector.rebuild",
				Principal: principal,
				Payload: map[string]any{
					"fieldId":      progress.FieldID,
					"batchIndex":   progress.BatchIndex,
					"totalBatches": progress.TotalBatches,
					"batchSize":    progress.BatchSize,
					"processed":    progress.Processed,
					"total":        progress.Total,
					"done":         progress.Done,
				},
			})
		})
	}
	return service.RebuildVector(fieldID, force)
}

func parseArgs(raw json.RawMessage, target any) error {
	rawText := strings.TrimSpace(string(raw))
	if len(raw) == 0 || rawText == "" || rawText == "null" {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid args: %w", err)
	}
	return nil
}

func normalizeToolRows(rows [][]any) [][]any {
	normalized := make([][]any, 0, len(rows))
	for _, row := range rows {
		next := make([]any, len(row))
		for i, value := range row {
			if b, ok := value.([]byte); ok {
				next[i] = string(b)
				continue
			}
			next[i] = value
		}
		normalized = append(normalized, next)
	}
	return normalized
}
