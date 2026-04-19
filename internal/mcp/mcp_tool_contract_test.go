package mcp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/report"
	"github.com/LynnColeArt/Quackcess/internal/terminal"
)

type fakeQueryRunner struct {
	calls  []string
	result terminal.TerminalResult
	err    error
}

func (f *fakeQueryRunner) RunCommand(input string) (terminal.TerminalResult, error) {
	f.calls = append(f.calls, input)
	if f.err != nil {
		return terminal.TerminalResult{}, f.err
	}
	return f.result, nil
}

type fakeCatalogService struct {
	tables   []string
	views    []string
	canvases []string
}

func (f *fakeCatalogService) ListTables() ([]string, error)   { return f.tables, nil }
func (f *fakeCatalogService) ListViews() ([]string, error)    { return f.views, nil }
func (f *fakeCatalogService) ListCanvases() ([]string, error) { return f.canvases, nil }

type fakeReportService struct {
	reportIDs           []string
	chartIDs            []string
	exportResult        report.ReportExport
	exportErr           error
	loadReportID        string
	loadReportChartData map[string][]report.ExportRow
}

func (f *fakeReportService) ListReportIDs() ([]string, error) { return f.reportIDs, nil }
func (f *fakeReportService) ListChartIDs() ([]string, error)  { return f.chartIDs, nil }
func (f *fakeReportService) LoadReportExport(reportID string, chartData map[string][]report.ExportRow) (report.ReportExport, error) {
	f.loadReportID = reportID
	f.loadReportChartData = chartData
	if f.exportErr != nil {
		return report.ReportExport{}, f.exportErr
	}
	return f.exportResult, nil
}

func newContractServer(t *testing.T) (*Server, *fakeQueryRunner, *fakeCatalogService, *MemoryArtifactStore) {
	t.Helper()
	queryRunner := &fakeQueryRunner{
		result: terminal.TerminalResult{
			Kind:                 terminal.TerminalKindQuery,
			SQLText:              "SELECT 1",
			Columns:              []string{"id"},
			Rows:                 [][]any{{int64(1)}, {int64(2)}},
			RowCount:             2,
			DurationMilliseconds: 17,
			ErrorText:            "",
		},
	}
	catalog := &fakeCatalogService{
		tables:   []string{"users", "orders"},
		views:    []string{"sales_view"},
		canvases: []string{"main"},
	}
	store := NewMemoryArtifactStore()

	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		QueryRunner:    queryRunner,
		CatalogService: catalog,
		Artifacts:      store,
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}
	return server, queryRunner, catalog, store
}

func newContractServerWithReports(t *testing.T) (*Server, *fakeReportService) {
	t.Helper()

	reportService := &fakeReportService{
		reportIDs: []string{"monthly", "weekly"},
		chartIDs:  []string{"trend", "distribution"},
		exportResult: report.ReportExport{
			ReportID: "monthly",
			Title:    "Monthly Snapshot",
			Sections: []report.ExportedReportSection{
				{ID: "intro", Kind: "text", Title: "Intro"},
				{ID: "chart", Kind: "chart", Title: "Trend"},
			},
		},
	}

	server := NewServer(NewAllowlistAuthorizer(true), nil)
	if err := RegisterCoreTools(server, CoreTools{
		Reports: reportService,
	}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}
	return server, reportService
}

func toolResultData(t *testing.T, result ToolResult) map[string]any {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result data type = %T, want map[string]any", result.Data)
	}
	return data
}

func asStringSlice(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]string)
	if !ok {
		t.Fatalf("unexpected type = %T, want []string", value)
	}
	return items
}

func TestRegisterCoreToolsRegistersExpectedToolNames(t *testing.T) {
	server, _, _, _ := newContractServer(t)
	got := server.ListToolNames()
	want := []string{
		"artifact.delete",
		"artifact.get",
		"artifact.list",
		"artifact.set",
		"query.execute",
		"schema.inspect",
		"system.ping",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestListToolsReturnsStableSortedDefinitions(t *testing.T) {
	server, _, _, _ := newContractServer(t)
	got := server.ListTools()
	gotNames := make([]string, 0, len(got))
	for _, tool := range got {
		gotNames = append(gotNames, tool.Name)
	}
	if !reflect.DeepEqual(gotNames, server.ListToolNames()) {
		t.Fatalf("tool names = %#v, want %#v", gotNames, server.ListToolNames())
	}
}

func TestCallToolSystemPingReturnsPrincipal(t *testing.T) {
	server, _, _, _ := newContractServer(t)
	result := server.CallTool(context.Background(), &CallRequest{
		Tool:      "system.ping",
		Principal: "alice",
	})
	if result.Tool != "system.ping" {
		t.Fatalf("tool = %q, want system.ping", result.Tool)
	}
	data := toolResultData(t, result)
	if data["pong"] != true {
		t.Fatalf("pong = %#v, want true", data["pong"])
	}
	if data["principal"] != "alice" {
		t.Fatalf("principal = %#v, want alice", data["principal"])
	}
}

func TestCallToolExecutesQueryAndNormalizesRows(t *testing.T) {
	server := NewServer(NewAllowlistAuthorizer(true), nil)
	queryRunner := &fakeQueryRunner{
		result: terminal.TerminalResult{
			Kind:                 terminal.TerminalKindQuery,
			SQLText:              "SELECT b FROM docs",
			Columns:              []string{"blob"},
			Rows:                 [][]any{{[]byte("hello-world")}},
			RowCount:             1,
			DurationMilliseconds: 11,
		},
	}
	if err := RegisterCoreTools(server, CoreTools{QueryRunner: queryRunner}); err != nil {
		t.Fatalf("register core tools: %v", err)
	}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "query.execute",
		Args: []byte(`{"sql":"SELECT b FROM docs"}`),
	})
	data := toolResultData(t, result)
	if got := data["kind"]; got != terminal.TerminalKindQuery {
		t.Fatalf("kind = %v, want %q", got, terminal.TerminalKindQuery)
	}
	if got := data["sql"]; got != "SELECT b FROM docs" {
		t.Fatalf("sql = %v, want SELECT b FROM docs", got)
	}
	rows, ok := data["rows"].([][]any)
	if !ok || len(rows) != 1 || rows[0][0] != "hello-world" {
		t.Fatalf("rows = %#v, want string-normalized blob", rows)
	}
}

func TestCallToolSchemaInspectReturnsSortedCatalogData(t *testing.T) {
	server, _, catalog, _ := newContractServer(t)
	catalog.tables = []string{"zebra", "alpha", "mango"}
	catalog.views = []string{"v2", "v1"}
	catalog.canvases = []string{"canvas-2", "canvas-1"}

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "schema.inspect",
	})
	data := toolResultData(t, result)
	if got := asStringSlice(t, data["tables"]); !reflect.DeepEqual(got, []string{"alpha", "mango", "zebra"}) {
		t.Fatalf("tables = %#v, want [alpha mango zebra]", got)
	}
	if got := asStringSlice(t, data["views"]); !reflect.DeepEqual(got, []string{"v1", "v2"}) {
		t.Fatalf("views = %#v, want [v1 v2]", got)
	}
	if got := asStringSlice(t, data["canvases"]); !reflect.DeepEqual(got, []string{"canvas-1", "canvas-2"}) {
		t.Fatalf("canvases = %#v, want [canvas-1 canvas-2]", got)
	}
}

func TestCallToolArtifactCrudRoundTrip(t *testing.T) {
	server, _, _, store := newContractServer(t)

	set := server.CallTool(context.Background(), &CallRequest{
		Tool: "artifact.set",
		Args: []byte(`{"id":"artifact-a","payload":"{\"kind\":\"chart\"}"}`),
	})
	if set.Error != nil {
		t.Fatalf("set error: %v", set.Error)
	}
	setData, ok := set.Data.(map[string]any)
	if !ok {
		t.Fatalf("set data type = %T", set.Data)
	}
	if got := setData["status"]; got != "updated" {
		t.Fatalf("set status = %q, want updated", setData["status"])
	}
	if got := setData["id"]; got != "artifact-a" {
		t.Fatalf("set id = %q, want artifact-a", setData["id"])
	}

	get := server.CallTool(context.Background(), &CallRequest{
		Tool: "artifact.get",
		Args: []byte(`{"id":"artifact-a"}`),
	})
	getData := toolResultData(t, get)
	if got := getData["payload"]; got != `{"kind":"chart"}` {
		t.Fatalf("get payload = %v, want payload", got)
	}

	list := server.CallTool(context.Background(), &CallRequest{
		Tool: "artifact.list",
	})
	listData := toolResultData(t, list)
	if got := asStringSlice(t, listData["items"]); !reflect.DeepEqual(got, []string{"artifact-a"}) {
		t.Fatalf("items = %#v, want [artifact-a]", got)
	}

	del := server.CallTool(context.Background(), &CallRequest{
		Tool: "artifact.delete",
		Args: []byte(`{"id":"artifact-a"}`),
	})
	delData := toolResultData(t, del)
	if got := delData["status"]; got != "deleted" {
		t.Fatalf("delete status = %q, want deleted", got)
	}
	if _, err := store.Get("artifact-a"); err == nil || !strings.Contains(err.Error(), "artifact not found") {
		t.Fatalf("artifact should be removed, err = %v", err)
	}
}

func TestRegisterCoreToolsRegistersReportingTools(t *testing.T) {
	server, _ := newContractServerWithReports(t)
	got := server.ListToolNames()
	want := []string{
		"chart.list",
		"report.export",
		"report.list",
		"system.ping",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestCallToolReportListReturnsReportIDs(t *testing.T) {
	server, reportService := newContractServerWithReports(t)

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "report.list",
	})
	data := toolResultData(t, result)
	if got := asStringSlice(t, data["items"]); !reflect.DeepEqual(got, reportService.reportIDs) {
		t.Fatalf("report ids = %#v, want %#v", got, reportService.reportIDs)
	}
}

func TestCallToolChartListReturnsChartIDs(t *testing.T) {
	server, reportService := newContractServerWithReports(t)

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "chart.list",
	})
	data := toolResultData(t, result)
	if got := asStringSlice(t, data["items"]); !reflect.DeepEqual(got, reportService.chartIDs) {
		t.Fatalf("chart ids = %#v, want %#v", got, reportService.chartIDs)
	}
}

func TestCallToolReportExportPassesReportIDAndChartData(t *testing.T) {
	server, reportService := newContractServerWithReports(t)

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "report.export",
		Args: []byte(`{"reportId":"monthly","chartData":{"trend":[{"month":"Jan","amount":1100},{"month":"Feb","amount":1200}]}}`),
	})
	data := toolResultData(t, result)

	if got := data["reportId"]; got != "monthly" {
		t.Fatalf("reportId = %#v, want monthly", got)
	}
	if got := data["title"]; got != "Monthly Snapshot" {
		t.Fatalf("title = %#v, want Monthly Snapshot", got)
	}

	if reportService.loadReportID != "monthly" {
		t.Fatalf("loadReportExport called with reportId = %#v, want monthly", reportService.loadReportID)
	}
	if got := reportService.loadReportChartData["trend"][0]["month"]; got != "Jan" {
		t.Fatalf("normalized chart row month = %#v, want Jan", got)
	}
	if got := reportService.loadReportChartData["trend"][1]["amount"]; got != float64(1200) {
		t.Fatalf("normalized chart row amount = %#v, want 1200", got)
	}
}

func TestReportExportRequiresReportID(t *testing.T) {
	server, _ := newContractServerWithReports(t)

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "report.export",
		Args: []byte(`{}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeInvalidArgument {
		t.Fatalf("error = %#v, want invalid_argument", result.Error)
	}
}

func TestReportExportSurfacesHandlerError(t *testing.T) {
	server, reportService := newContractServerWithReports(t)
	reportService.exportErr = fmt.Errorf("cannot render report")

	result := server.CallTool(context.Background(), &CallRequest{
		Tool: "report.export",
		Args: []byte(`{"reportId":"monthly"}`),
	})
	if result.Error == nil || result.Error.Code != ErrorCodeHandlerError {
		t.Fatalf("error = %#v, want handler_error", result.Error)
	}
	if !strings.Contains(result.Error.Message, "cannot render report") {
		t.Fatalf("error message = %q", result.Error.Message)
	}
}
