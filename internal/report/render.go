package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	mimeMarkdown      = "text/markdown"
	mimeVegaLiteJSON  = "application/vnd.vegalite+json"
	renderImagePrefix = "quackcess://render/"
)

type RenderedChart struct {
	ID        string `json:"id"`
	Renderer  string `json:"renderer"`
	ChartType string `json:"chartType"`
	Title     string `json:"title,omitempty"`
	MimeType  string `json:"mimeType"`
	Content   string `json:"content"`
}

type RenderedReportSection struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Title    string         `json:"title,omitempty"`
	Text     string         `json:"text,omitempty"`
	Rendered *RenderedChart `json:"rendered,omitempty"`
}

type ReportRenderPlan struct {
	ID       string                  `json:"id"`
	Title    string                  `json:"title,omitempty"`
	Sections []RenderedReportSection `json:"sections"`
}

type ExportRow map[string]interface{}

type ExportedReportSection struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title,omitempty"`
	Text   string `json:"text,omitempty"`
	Render string `json:"render"`
	CSV    string `json:"csv,omitempty"`
	Image  string `json:"image"`
}

type ReportExport struct {
	ReportID string                  `json:"reportId"`
	Title    string                  `json:"title,omitempty"`
	Sections []ExportedReportSection `json:"sections"`
}

// RenderChart converts a chart spec into an output payload that can be passed
// to chart renderers or export pipelines.
func RenderChart(rawSpec ChartSpec) (RenderedChart, error) {
	spec, err := CanonicalizeChartSpec(rawSpec)
	if err != nil {
		return RenderedChart{}, err
	}

	switch spec.Renderer {
	case ChartRendererMermaid:
		return renderMermaidChart(spec), nil
	case ChartRendererVegaLite:
		return renderVegaLiteChart(spec)
	default:
		return RenderedChart{}, fmt.Errorf("unsupported chart renderer: %s", spec.Renderer)
	}
}

// RenderReport resolves chart sections into rendered chart content.
func RenderReport(reportRaw ReportSpec, chartSpecs map[string]ChartSpec) (ReportRenderPlan, error) {
	report, err := CanonicalizeReportSpec(reportRaw)
	if err != nil {
		return ReportRenderPlan{}, err
	}

	sections := make([]RenderedReportSection, 0, len(report.Sections))
	for _, section := range report.Sections {
		renderedSection := RenderedReportSection{
			ID:    section.ID,
			Kind:  section.Kind,
			Title: section.Title,
			Text:  section.Text,
		}
		if section.Kind == ReportSectionChart {
			chartSpec, ok := chartSpecs[section.ChartID]
			if !ok {
				return ReportRenderPlan{}, fmt.Errorf("report chart section refers to missing chart: %s", section.ChartID)
			}
			rendered, err := RenderChart(chartSpec)
			if err != nil {
				return ReportRenderPlan{}, err
			}
			renderedSection.Rendered = &rendered
		}
		sections = append(sections, renderedSection)
	}
	return ReportRenderPlan{
		ID:       report.ID,
		Title:    report.Title,
		Sections: sections,
	}, nil
}

// ExportReport converts a render plan into deterministic export sections and CSV blobs.
func ExportReport(plan ReportRenderPlan, chartData map[string][]ExportRow) (ReportExport, error) {
	if plan.ID == "" {
		return ReportExport{}, fmt.Errorf("report id is required")
	}
	if chartData == nil {
		chartData = map[string][]ExportRow{}
	}
	exported := make([]ExportedReportSection, 0, len(plan.Sections))
	for _, section := range plan.Sections {
		exportedSection := ExportedReportSection{
			ID:    section.ID,
			Kind:  section.Kind,
			Title: section.Title,
			Text:  section.Text,
			Image: "",
		}
		switch section.Kind {
		case ReportSectionChart:
			if section.Rendered == nil {
				return ReportExport{}, fmt.Errorf("report chart section missing rendered chart: %s", section.ID)
			}
			exportedSection.Render = section.Rendered.Content
			exportedSection.Image = renderImage(section.Rendered.ID)
			if rows, ok := chartData[section.Rendered.ID]; ok && len(rows) > 0 {
				csvText, err := RenderRowsAsCSV(rows)
				if err != nil {
					return ReportExport{}, err
				}
				exportedSection.CSV = csvText
			}
		case ReportSectionText:
			// no output payload needed.
		default:
			return ReportExport{}, fmt.Errorf("unsupported report section kind: %s", section.Kind)
		}
		exported = append(exported, exportedSection)
	}

	return ReportExport{
		ReportID: plan.ID,
		Title:    plan.Title,
		Sections: exported,
	}, nil
}

// RenderRowsAsCSV materializes rows into a deterministic CSV string.
func RenderRowsAsCSV(rows []ExportRow) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	cols := collectSortedColumns(rows)
	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)
	if err := writer.Write(cols); err != nil {
		return "", err
	}
	for _, row := range rows {
		record := make([]string, len(cols))
		for i, col := range cols {
			if row == nil {
				record[i] = ""
				continue
			}
			value, ok := row[col]
			if !ok {
				record[i] = ""
				continue
			}
			record[i] = formatExportValue(value)
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func renderMermaidChart(spec ChartSpec) RenderedChart {
	var body string
	if def := strings.TrimSpace(string(spec.Definition)); def != "" {
		body = def
	}
	if spec.Source != "" {
		if body != "" {
			body = body + "\n"
		}
		body += spec.Source
	}
	if body == "" {
		body = "%% " + spec.ChartType
	}
	content := "```mermaid\n" + strings.TrimSpace(body) + "\n```"
	return RenderedChart{
		ID:        spec.ID,
		Renderer:  ChartRendererMermaid,
		ChartType: spec.ChartType,
		Title:     spec.Title,
		MimeType:  mimeMarkdown,
		Content:   content,
	}
}

func renderVegaLiteChart(spec ChartSpec) (RenderedChart, error) {
	definition := bytes.TrimSpace(spec.Definition)
	if len(definition) == 0 {
		return RenderedChart{}, fmt.Errorf("vega-light chart definition is required")
	}
	if !json.Valid(definition) {
		return RenderedChart{}, fmt.Errorf("invalid vega-light definition: not valid JSON")
	}
	return RenderedChart{
		ID:        spec.ID,
		Renderer:  ChartRendererVegaLite,
		ChartType: spec.ChartType,
		Title:     spec.Title,
		MimeType:  mimeVegaLiteJSON,
		Content:   string(definition),
	}, nil
}

func renderImage(chartID string) string {
	return renderImagePrefix + chartID + ".svg"
}

func collectSortedColumns(rows []ExportRow) []string {
	seen := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			seen[key] = struct{}{}
		}
	}
	cols := make([]string, 0, len(seen))
	for col := range seen {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

func formatExportValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}
