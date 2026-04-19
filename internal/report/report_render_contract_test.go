package report

import (
	"strings"
	"testing"
)

func TestRenderReportResolvesChartSectionsAndRenders(t *testing.T) {
	chartSpec := ChartSpec{
		ID:            "chart-sales",
		Kind:          "chart",
		Renderer:      "mermaid",
		ChartType:     "line",
		SourceType:    "query",
		Source:        "SELECT date, revenue FROM sales",
		Definition:    jsonRaw(`flowchart TD; A-->B`),
		SchemaVersion: CurrentChartSchemaVersion(),
	}
	reportSpec := ReportSpec{
		ID:            "report-1",
		Kind:          "report",
		SchemaVersion: CurrentReportSchemaVersion(),
		Title:         "Sales report",
		Sections: []ReportSection{
			{ID: "heading", Kind: "text", Text: "summary"},
			{ID: "main", Kind: "chart", ChartID: "chart-sales"},
		},
	}

	plan, err := RenderReport(reportSpec, map[string]ChartSpec{
		"chart-sales": chartSpec,
	})
	if err != nil {
		t.Fatalf("render report: %v", err)
	}
	if plan.ID != reportSpec.ID {
		t.Fatalf("plan id = %q, want %q", plan.ID, reportSpec.ID)
	}
	if len(plan.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(plan.Sections))
	}
	if plan.Sections[0].Kind != ReportSectionText {
		t.Fatalf("section 0 kind = %q, want %q", plan.Sections[0].Kind, ReportSectionText)
	}
	if plan.Sections[1].Kind != ReportSectionChart {
		t.Fatalf("section 1 kind = %q, want %q", plan.Sections[1].Kind, ReportSectionChart)
	}
	if plan.Sections[1].Rendered == nil {
		t.Fatal("missing rendered chart payload")
	}
	if plan.Sections[1].Rendered.ID != "chart-sales" {
		t.Fatalf("rendered id = %q, want %q", plan.Sections[1].Rendered.ID, "chart-sales")
	}
	if !strings.Contains(plan.Sections[1].Rendered.Content, "SELECT date, revenue FROM sales") {
		t.Fatalf("rendered content mismatch: %q", plan.Sections[1].Rendered.Content)
	}
}

func TestRenderReportFailsWhenChartIsMissing(t *testing.T) {
	reportSpec := ReportSpec{
		ID:            "report-missing",
		Kind:          "report",
		SchemaVersion: CurrentReportSchemaVersion(),
		Sections: []ReportSection{
			{ID: "main", Kind: "chart", ChartID: "missing-chart"},
		},
	}

	_, err := RenderReport(reportSpec, map[string]ChartSpec{})
	if err == nil {
		t.Fatal("expected missing chart error")
	}
}

func TestRenderChartProducesVegaLitePayload(t *testing.T) {
	spec := ChartSpec{
		ID:            "vchart",
		Kind:          "chart",
		Renderer:      "vega-light",
		ChartType:     "bar",
		SourceType:    "query",
		Source:        "SELECT category, count(*) FROM events",
		Definition:    jsonRaw(`{"mark":"bar","encoding":{"x":{"field":"category","type":"nominal"}}}`),
		SchemaVersion: CurrentChartSchemaVersion(),
	}

	rendered, err := RenderChart(spec)
	if err != nil {
		t.Fatalf("render chart: %v", err)
	}
	if rendered.Renderer != ChartRendererVegaLite {
		t.Fatalf("renderer = %q, want %q", rendered.Renderer, ChartRendererVegaLite)
	}
	if !strings.Contains(rendered.Content, `"mark":"bar"`) {
		t.Fatalf("content = %q", rendered.Content)
	}
	if rendered.MimeType != "application/vnd.vegalite+json" {
		t.Fatalf("mime = %q, want application/vnd.vegalite+json", rendered.MimeType)
	}
}

func TestRenderVegaLiteChartRejectsInvalidDefinition(t *testing.T) {
	spec := ChartSpec{
		ID:            "bad",
		Kind:          "chart",
		Renderer:      "vega-light",
		ChartType:     "bar",
		SourceType:    "query",
		Source:        "SELECT 1",
		Definition:    jsonRaw(`{bad`),
		SchemaVersion: CurrentChartSchemaVersion(),
	}
	if _, err := RenderChart(spec); err == nil {
		t.Fatal("expected invalid definition error")
	}
}

func jsonRaw(s string) []byte {
	return []byte(s)
}
