package report

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	chartSpecSchemaVersion = "1.0.0"
	reportSpecSchemaVersion = "1.0.0"
)

const (
	ChartRendererMermaid  = "mermaid"
	ChartRendererVegaLite = "vega-light"
)

const (
	ChartSourceQuery = "query"
	ChartSourceSQL   = "sql"
)

const (
	ReportSectionChart = "chart"
	ReportSectionText  = "text"
)

type ChartSpec struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	SchemaVersion string          `json:"schemaVersion"`
	Title        string          `json:"title,omitempty"`
	Description  string          `json:"description,omitempty"`
	Renderer     string          `json:"renderer"`
	ChartType    string          `json:"chartType"`
	SourceType   string          `json:"sourceType"`
	Source       string          `json:"source"`
	Definition   json.RawMessage `json:"definition,omitempty"`
	Parameters   []string        `json:"parameters,omitempty"`
}

type ReportSection struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	ChartID string `json:"chartId,omitempty"`
	Text    string `json:"text,omitempty"`
}

type ReportSpec struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	SchemaVersion string         `json:"schemaVersion"`
	Title         string         `json:"title,omitempty"`
	Description   string         `json:"description,omitempty"`
	Sections      []ReportSection `json:"sections"`
}

// CurrentChartSchemaVersion is the supported chart schema version.
func CurrentChartSchemaVersion() string {
	return chartSpecSchemaVersion
}

// CurrentReportSchemaVersion is the supported report schema version.
func CurrentReportSchemaVersion() string {
	return reportSpecSchemaVersion
}

func ParseChartSpec(raw []byte) (ChartSpec, error) {
	var spec ChartSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return ChartSpec{}, fmt.Errorf("invalid chart spec JSON: %w", err)
	}
	return CanonicalizeChartSpec(spec)
}

func ParseReportSpec(raw []byte) (ReportSpec, error) {
	var spec ReportSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return ReportSpec{}, fmt.Errorf("invalid report spec JSON: %w", err)
	}
	return CanonicalizeReportSpec(spec)
}

func CanonicalizeChartSpec(spec ChartSpec) (ChartSpec, error) {
	spec.ID = strings.TrimSpace(spec.ID)
	spec.Kind = strings.TrimSpace(spec.Kind)
	spec.SchemaVersion = strings.TrimSpace(spec.SchemaVersion)
	spec.Title = strings.TrimSpace(spec.Title)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.Renderer = strings.ToLower(strings.TrimSpace(spec.Renderer))
	spec.ChartType = strings.TrimSpace(spec.ChartType)
	spec.SourceType = strings.ToLower(strings.TrimSpace(spec.SourceType))
	spec.Source = strings.TrimSpace(spec.Source)

	if spec.Kind == "" {
		spec.Kind = "chart"
	}
	if spec.SchemaVersion == "" {
		spec.SchemaVersion = CurrentChartSchemaVersion()
	}

	if err := validateChartSpec(spec); err != nil {
		return ChartSpec{}, err
	}
	return spec, nil
}

func CanonicalizeReportSpec(spec ReportSpec) (ReportSpec, error) {
	spec.ID = strings.TrimSpace(spec.ID)
	spec.Kind = strings.TrimSpace(spec.Kind)
	spec.SchemaVersion = strings.TrimSpace(spec.SchemaVersion)
	spec.Title = strings.TrimSpace(spec.Title)
	spec.Description = strings.TrimSpace(spec.Description)

	if spec.Kind == "" {
		spec.Kind = "report"
	}
	if spec.SchemaVersion == "" {
		spec.SchemaVersion = CurrentReportSchemaVersion()
	}

	if spec.Sections == nil {
		spec.Sections = []ReportSection{}
	}
	for i, section := range spec.Sections {
		section.ID = strings.TrimSpace(section.ID)
		section.Kind = strings.TrimSpace(section.Kind)
		section.Title = strings.TrimSpace(section.Title)
		section.ChartID = strings.TrimSpace(section.ChartID)
		section.Text = strings.TrimSpace(section.Text)
		spec.Sections[i] = section
	}
	if err := validateReportSpec(spec); err != nil {
		return ReportSpec{}, err
	}
	return spec, nil
}

func MarshalChartSpec(spec ChartSpec) ([]byte, error) {
	canonical, err := CanonicalizeChartSpec(spec)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

func MarshalReportSpec(spec ReportSpec) ([]byte, error) {
	canonical, err := CanonicalizeReportSpec(spec)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

func validateChartSpec(spec ChartSpec) error {
	if strings.TrimSpace(spec.ID) == "" {
		return fmt.Errorf("chart id is required")
	}
	if spec.Kind != "chart" {
		return fmt.Errorf("chart kind is not supported: %s", spec.Kind)
	}
	if strings.TrimSpace(spec.SchemaVersion) == "" {
		return fmt.Errorf("chart schemaVersion is required")
	}
	if spec.SchemaVersion != CurrentChartSchemaVersion() {
		return fmt.Errorf("unsupported chart schema version: %s", spec.SchemaVersion)
	}
	if strings.TrimSpace(spec.Renderer) == "" {
		return fmt.Errorf("chart renderer is required")
	}
	switch spec.Renderer {
	case ChartRendererMermaid, ChartRendererVegaLite:
	default:
		return fmt.Errorf("unsupported chart renderer: %s", spec.Renderer)
	}
	if strings.TrimSpace(spec.ChartType) == "" {
		return fmt.Errorf("chart type is required")
	}
	if strings.TrimSpace(spec.SourceType) == "" {
		return fmt.Errorf("chart sourceType is required")
	}
	switch spec.SourceType {
	case ChartSourceQuery, ChartSourceSQL:
	default:
		return fmt.Errorf("unsupported chart sourceType: %s", spec.SourceType)
	}
	if strings.TrimSpace(spec.Source) == "" {
		return fmt.Errorf("chart source is required")
	}
	return nil
}

func validateReportSpec(spec ReportSpec) error {
	if strings.TrimSpace(spec.ID) == "" {
		return fmt.Errorf("report id is required")
	}
	if spec.Kind != "report" {
		return fmt.Errorf("report kind is not supported: %s", spec.Kind)
	}
	if strings.TrimSpace(spec.SchemaVersion) == "" {
		return fmt.Errorf("report schemaVersion is required")
	}
	if spec.SchemaVersion != CurrentReportSchemaVersion() {
		return fmt.Errorf("unsupported report schema version: %s", spec.SchemaVersion)
	}
	seenSections := map[string]struct{}{}
	for _, section := range spec.Sections {
		if section.ID != "" {
			if _, ok := seenSections[section.ID]; ok {
				return fmt.Errorf("duplicate report section id: %s", section.ID)
			}
			seenSections[section.ID] = struct{}{}
		}
		if section.Kind == "" {
			return fmt.Errorf("report section kind is required")
		}
		switch section.Kind {
		case ReportSectionChart, ReportSectionText:
		default:
			return fmt.Errorf("unsupported report section kind: %s", section.Kind)
		}
		if section.Kind == ReportSectionChart && section.ChartID == "" {
			return fmt.Errorf("report chart section requires chartId")
		}
		if section.Kind == ReportSectionText && section.Text == "" {
			return fmt.Errorf("report text section requires text")
		}
	}
	return nil
}
