package project

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"reflect"
	"testing"

	"github.com/LynnColeArt/Quackcess/internal/report"
)

func TestProjectCanLoadChartAndReportSpecsFromArtifacts(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "analytics.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "Analytics"
	manifest.CreatedBy = "tester"

	chartSpec, err := report.MarshalChartSpec(report.ChartSpec{
		ID:            "trend-chart",
		Kind:          "chart",
		SchemaVersion: report.CurrentChartSchemaVersion(),
		Title:         "Trend",
		Renderer:      report.ChartRendererMermaid,
		ChartType:     "line",
		SourceType:    report.ChartSourceQuery,
		Source:        "SELECT date, amount FROM monthly_sales ORDER BY date",
		Definition:    json.RawMessage(`"flowchart TD\n  A --> B"`),
	})
	if err != nil {
		t.Fatalf("marshal chart spec: %v", err)
	}

	reportSpec, err := report.MarshalReportSpec(report.ReportSpec{
		ID:            "executive-report",
		Kind:          "report",
		SchemaVersion: report.CurrentReportSchemaVersion(),
		Title:         "Executive Report",
		Sections: []report.ReportSection{
			{
				ID:    "intro",
				Kind:  report.ReportSectionText,
				Title: "Overview",
				Text:  "Summary of sales activity",
			},
			{
				ID:      "chart-section",
				Kind:    report.ReportSectionChart,
				Title:   "Sales Trend",
				ChartID: "trend-chart",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal report spec: %v", err)
	}

	artifacts := map[string][]byte{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "trend-chart"):       chartSpec,
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "executive-report"): reportSpec,
	}

	if err := Create(projectPath, CreateOptions{
		Manifest:  manifest,
		Artifacts: artifacts,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	loadedChart, err := project.ReadChartSpec("trend-chart")
	if err != nil {
		t.Fatalf("read chart spec: %v", err)
	}
	if loadedChart.ID != "trend-chart" {
		t.Fatalf("loaded chart id = %q, want trend-chart", loadedChart.ID)
	}

	loadedReport, err := project.ReadReportSpec("executive-report")
	if err != nil {
		t.Fatalf("read report spec: %v", err)
	}
	if loadedReport.ID != "executive-report" {
		t.Fatalf("loaded report id = %q, want executive-report", loadedReport.ID)
	}
	if len(loadedReport.Sections) != 2 {
		t.Fatalf("loaded report sections = %d, want 2", len(loadedReport.Sections))
	}

	plan, err := project.LoadReportRenderPlan("executive-report")
	if err != nil {
		t.Fatalf("load report render plan: %v", err)
	}
	if plan.ID != "executive-report" {
		t.Fatalf("plan id = %q, want executive-report", plan.ID)
	}
	if len(plan.Sections) != 2 {
		t.Fatalf("plan sections = %d, want 2", len(plan.Sections))
	}
	if plan.Sections[1].Rendered == nil {
		t.Fatalf("expected chart section to be rendered")
	}
	if plan.Sections[1].Rendered.ID != "trend-chart" {
		t.Fatalf("rendered chart id = %q, want trend-chart", plan.Sections[1].Rendered.ID)
	}
	if !strings.Contains(plan.Sections[1].Rendered.Content, "```mermaid") {
		t.Fatalf("rendered chart content missing mermaid block: %q", plan.Sections[1].Rendered.Content)
	}
}

func TestProjectLoadReportRenderPlanFailsForMissingChartDependency(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "broken.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "Broken"
	manifest.CreatedBy = "tester"

	reportSpec, err := report.MarshalReportSpec(report.ReportSpec{
		ID:            "broken-report",
		Kind:          "report",
		SchemaVersion: report.CurrentReportSchemaVersion(),
		Sections: []report.ReportSection{
			{
				ID:      "chart",
				Kind:    report.ReportSectionChart,
				ChartID: "does-not-exist",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal report spec: %v", err)
	}

	err = Create(projectPath, CreateOptions{
		Manifest: manifest,
		Artifacts: map[string][]byte{
			ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "broken-report"): reportSpec,
		},
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	_, err = project.LoadReportRenderPlan("broken-report")
	if err == nil {
		t.Fatal("expected missing chart error")
	}
	if !strings.Contains(err.Error(), "report chart section refers to missing chart: does-not-exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectListArtifactsByKindReturnsDeterministicOrder(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "kinds.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "Kinds"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "c2"):  []byte(`{"id":"c2","kind":"chart","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "c1"):  []byte(`{"id":"c1","kind":"chart","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "r1"): []byte(`{"id":"r1","kind":"report","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindQuery, "q1"):  []byte(`{"id":"q1","kind":"query","schemaVersion":"1.0.0"}`),
	}

	err := Create(projectPath, CreateOptions{
		Manifest:  manifest,
		Artifacts: artifacts,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	entries, err := project.ListArtifactsByKind(ArtifactKindChart)
	if err != nil {
		t.Fatalf("list chart artifacts: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("chart entries = %d, want 2", len(entries))
	}
	if entries[0].ID != "c1" || entries[1].ID != "c2" {
		t.Fatalf("chart entries order = [%s %s], want [c1 c2]", entries[0].ID, entries[1].ID)
	}
}

func TestProjectListChartAndReportIDsIsDeterministic(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "ids.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "IDs"
	manifest.CreatedBy = "tester"

	artifacts := map[string][]byte{
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "r2"): []byte(`{"id":"r2","kind":"report","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "r1"): []byte(`{"id":"r1","kind":"report","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "c2"):  []byte(`{"id":"c2","kind":"chart","schemaVersion":"1.0.0"}`),
		ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "c1"):  []byte(`{"id":"c1","kind":"chart","schemaVersion":"1.0.0"}`),
	}

	if err := Create(projectPath, CreateOptions{Manifest: manifest, Artifacts: artifacts}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	reportIDs, err := project.ListReportIDs()
	if err != nil {
		t.Fatalf("list report ids: %v", err)
	}
	if !reflect.DeepEqual(reportIDs, []string{"r1", "r2"}) {
		t.Fatalf("report ids = %#v, want [r1 r2]", reportIDs)
	}

	chartIDs, err := project.ListChartIDs()
	if err != nil {
		t.Fatalf("list chart ids: %v", err)
	}
	if !reflect.DeepEqual(chartIDs, []string{"c1", "c2"}) {
		t.Fatalf("chart ids = %#v, want [c1 c2]", chartIDs)
	}
}

func TestProjectCanExportReportFromArtifactAndRows(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "export.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "Export"
	manifest.CreatedBy = "tester"

	chartSpec, err := report.MarshalChartSpec(report.ChartSpec{
		ID:            "trend-chart",
		Kind:          "chart",
		SchemaVersion: report.CurrentChartSchemaVersion(),
		Renderer:      report.ChartRendererMermaid,
		ChartType:     "line",
		SourceType:    report.ChartSourceQuery,
		Source:        "SELECT month, amount FROM monthly_sales",
		Definition:    json.RawMessage(`"flowchart TD\n  A --> B"`),
	})
	if err != nil {
		t.Fatalf("marshal chart spec: %v", err)
	}

	reportSpec, err := report.MarshalReportSpec(report.ReportSpec{
		ID:            "monthly-report",
		Kind:          "report",
		SchemaVersion: report.CurrentReportSchemaVersion(),
		Sections: []report.ReportSection{
			{
				ID:    "title",
				Kind:  report.ReportSectionText,
				Title: "Monthly report",
				Text:  "Overview of monthly sales.",
			},
			{
				ID:      "chart",
				Kind:    report.ReportSectionChart,
				Title:   "Sales trend",
				ChartID: "trend-chart",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal report spec: %v", err)
	}

	if err := Create(projectPath, CreateOptions{
		Manifest: manifest,
		Artifacts: map[string][]byte{
			ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindChart, "trend-chart"):     chartSpec,
			ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "monthly-report"): reportSpec,
		},
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	exported, err := project.LoadReportExport("monthly-report", map[string][]report.ExportRow{
		"trend-chart": {
			{"month": "Jan", "amount": int64(110)},
			{"month": "Feb", "amount": int64(130)},
		},
	})
	if err != nil {
		t.Fatalf("load report export: %v", err)
	}

	if exported.ReportID != "monthly-report" {
		t.Fatalf("report id = %q, want monthly-report", exported.ReportID)
	}
	if len(exported.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(exported.Sections))
	}
	if exported.Sections[0].CSV != "" {
		t.Fatalf("unexpected csv on text section = %q", exported.Sections[0].CSV)
	}
	if !strings.Contains(exported.Sections[1].Image, "quackcess://render/trend-chart.svg") {
		t.Fatalf("image = %q", exported.Sections[1].Image)
	}
	if !strings.Contains(exported.Sections[1].CSV, "amount,month") {
		t.Fatalf("csv = %q", exported.Sections[1].CSV)
	}
}

func TestProjectLoadReportExportPropagatesMissingChartError(t *testing.T) {
	tmp := t.TempDir()
	projectPath := filepath.Join(tmp, "export-missing.qdb")

	manifest := DefaultManifest()
	manifest.ProjectName = "ExportMissing"
	manifest.CreatedBy = "tester"

	reportSpec, err := report.MarshalReportSpec(report.ReportSpec{
		ID:            "broken-report",
		Kind:          "report",
		SchemaVersion: report.CurrentReportSchemaVersion(),
		Sections: []report.ReportSection{
			{
				ID:      "chart",
				Kind:    report.ReportSectionChart,
				Title:   "Missing",
				ChartID: "never-there",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal report spec: %v", err)
	}

	if err := Create(projectPath, CreateOptions{
		Manifest: manifest,
		Artifacts: map[string][]byte{
			ArtifactManifestPath(manifest.ArtifactRoot, ArtifactKindReport, "broken-report"): reportSpec,
		},
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	project, err := Open(projectPath)
	if err != nil {
		t.Fatalf("open project: %v", err)
	}

	_, err = project.LoadReportExport("broken-report", nil)
	if err == nil {
		t.Fatal("expected missing chart error")
	}
	if !strings.Contains(err.Error(), "report chart section refers to missing chart: never-there") {
		t.Fatalf("unexpected error: %v", err)
	}
}
