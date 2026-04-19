package report

import (
	"strings"
	"testing"
)

func TestExportReportCreatesCSVAndImagePlaceholder(t *testing.T) {
	chartRendered := RenderedChart{
		ID:        "chart-sales",
		Renderer:  ChartRendererMermaid,
		ChartType: "line",
		MimeType:  "text/markdown",
		Content:   "```mermaid\nflowchart TD\n  A-->B\n```",
	}
	plan := ReportRenderPlan{
		ID: "report-export",
		Sections: []RenderedReportSection{
			{
				ID:    "intro",
				Kind:  ReportSectionText,
				Title: "Intro",
				Text:  "Summary",
			},
			{
				ID:       "chart",
				Kind:     ReportSectionChart,
				Rendered: &chartRendered,
			},
		},
	}
	rows := []ExportRow{
		{"month": "Jan", "revenue": 100},
		{"month": "Feb", "revenue": 175},
	}

	exported, err := ExportReport(plan, map[string][]ExportRow{
		"chart-sales": rows,
	})
	if err != nil {
		t.Fatalf("export report: %v", err)
	}
	if len(exported.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(exported.Sections))
	}
	if got := exported.Sections[1].CSV; got == "" {
		t.Fatal("expected csv output")
	}
	if got := exported.Sections[1].Image; got != "quackcess://render/chart-sales.svg" {
		t.Fatalf("image = %q, want %q", got, "quackcess://render/chart-sales.svg")
	}
	if !containsLine(exported.Sections[1].CSV, "month,revenue") {
		t.Fatalf("csv = %q", exported.Sections[1].CSV)
	}
}

func TestExportReportPreservesDeterministicCsvColumnOrder(t *testing.T) {
	rows := []ExportRow{
		{"beta": "2", "alpha": "1"},
		{"alpha": "3", "beta": "4"},
	}

	csvText, err := RenderRowsAsCSV(rows)
	if err != nil {
		t.Fatalf("render rows: %v", err)
	}
	if !containsLine(csvText, "alpha,beta") {
		t.Fatalf("unexpected header order: %q", csvText)
	}
	if !containsLine(csvText, "1,2") {
		t.Fatalf("row 1 mismatch: %q", csvText)
	}
	if !containsLine(csvText, "3,4") {
		t.Fatalf("row 2 mismatch: %q", csvText)
	}
}

func TestExportReportWithoutRowsOmitsCsvForChartSections(t *testing.T) {
	chartRendered := RenderedChart{
		ID:        "chart-empty",
		Renderer:  ChartRendererMermaid,
		ChartType: "line",
		MimeType:  "text/markdown",
		Content:   "ok",
	}
	plan := ReportRenderPlan{
		ID: "report-empty",
		Sections: []RenderedReportSection{
			{
				ID:       "chart",
				Kind:     ReportSectionChart,
				Rendered: &chartRendered,
			},
		},
	}

	exported, err := ExportReport(plan, nil)
	if err != nil {
		t.Fatalf("export report: %v", err)
	}
	if exported.Sections[0].CSV != "" {
		t.Fatalf("expected empty csv, got %q", exported.Sections[0].CSV)
	}
	if exported.Sections[0].Image != "quackcess://render/chart-empty.svg" {
		t.Fatalf("image = %q, want %q", exported.Sections[0].Image, "quackcess://render/chart-empty.svg")
	}
}

func containsLine(value string, expected string) bool {
	return strings.Contains(value, expected)
}
