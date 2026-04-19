package project

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/LynnColeArt/Quackcess/internal/report"
)

// ReadArtifactSpec parses the manifest for a specific artifact kind/id.
func (p *Project) ReadArtifactSpec(kind ArtifactKind, id string) (ArtifactSpecV1, error) {
	if _, err := NewArtifactKind(string(kind)); err != nil {
		return ArtifactSpecV1{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ArtifactSpecV1{}, fmt.Errorf("artifact id is required")
	}

	manifestPath := ArtifactManifestPath(p.Manifest.ArtifactRoot, kind, id)
	raw, err := p.ReadArtifact(manifestPath)
	if err != nil {
		return ArtifactSpecV1{}, err
	}
	spec, err := ParseArtifactSpec(raw)
	if err != nil {
		return ArtifactSpecV1{}, err
	}
	if spec.Kind != kind {
		return ArtifactSpecV1{}, fmt.Errorf("artifact kind mismatch for %s: %s", manifestPath, spec.Kind)
	}
	return spec, nil
}

// ListArtifactsByKind returns deterministic entries for all artifacts of a kind.
func (p *Project) ListArtifactsByKind(kind ArtifactKind) ([]ArtifactIndexEntry, error) {
	if _, err := NewArtifactKind(string(kind)); err != nil {
		return nil, err
	}
	entries, err := p.ArtifactIndex()
	if err != nil {
		return nil, err
	}
	out := make([]ArtifactIndexEntry, 0)
	for _, entry := range entries {
		if entry.Kind == kind {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// ListReportIDs returns report artifact ids sorted by id.
func (p *Project) ListReportIDs() ([]string, error) {
	reports, err := p.ListArtifactsByKind(ArtifactKindReport)
	if err != nil {
		return nil, err
	}
	idList := make([]string, 0, len(reports))
	for _, entry := range reports {
		idList = append(idList, entry.ID)
	}
	return idList, nil
}

// ListChartIDs returns chart artifact ids sorted by id.
func (p *Project) ListChartIDs() ([]string, error) {
	charts, err := p.ListArtifactsByKind(ArtifactKindChart)
	if err != nil {
		return nil, err
	}
	idList := make([]string, 0, len(charts))
	for _, entry := range charts {
		idList = append(idList, entry.ID)
	}
	return idList, nil
}

// ArtifactIndex loads all artifact manifests in the project and validates their shape.
func (p *Project) ArtifactIndex() (map[string]ArtifactIndexEntry, error) {
	zfile, err := zip.OpenReader(p.Path)
	if err != nil {
		return nil, err
	}
	defer zfile.Close()

	payloads := map[string][]byte{}
	for _, f := range zfile.File {
		cleanPath, err := normalizedArtifactPath(p.Manifest.ArtifactRoot, f.Name)
		if err != nil {
			continue
		}
		if path.Base(cleanPath) != artifactManifestFile {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return nil, err
		}
		payloads[cleanPath] = data
	}
	return BuildArtifactIndex(p.Manifest.ArtifactRoot, payloads)
}

// ReadChartSpec loads and parses a chart artifact from the project.
func (p *Project) ReadChartSpec(id string) (report.ChartSpec, error) {
	raw, err := p.readKindArtifactPayload(ArtifactKindChart, id)
	if err != nil {
		return report.ChartSpec{}, err
	}
	return report.ParseChartSpec(raw)
}

// ReadReportSpec loads and parses a report artifact from the project.
func (p *Project) ReadReportSpec(id string) (report.ReportSpec, error) {
	raw, err := p.readKindArtifactPayload(ArtifactKindReport, id)
	if err != nil {
		return report.ReportSpec{}, err
	}
	return report.ParseReportSpec(raw)
}

// LoadReportRenderPlan loads a report and resolves all chart references.
func (p *Project) LoadReportRenderPlan(reportID string) (report.ReportRenderPlan, error) {
	reportSpec, err := p.ReadReportSpec(reportID)
	if err != nil {
		return report.ReportRenderPlan{}, err
	}

	chartSpecs := map[string]report.ChartSpec{}
	for _, section := range reportSpec.Sections {
		if section.Kind != report.ReportSectionChart {
			continue
		}
		if _, ok := chartSpecs[section.ChartID]; ok {
			continue
		}
		chartSpec, err := p.ReadChartSpec(section.ChartID)
		if err != nil {
			return report.ReportRenderPlan{}, fmt.Errorf("report chart section refers to missing chart: %s", section.ChartID)
		}
		chartSpecs[section.ChartID] = chartSpec
	}

	return report.RenderReport(reportSpec, chartSpecs)
}

// LoadReportExport resolves a report, renders all sections, and exports chart data as CSV/image placeholders.
func (p *Project) LoadReportExport(reportID string, chartData map[string][]report.ExportRow) (report.ReportExport, error) {
	plan, err := p.LoadReportRenderPlan(reportID)
	if err != nil {
		return report.ReportExport{}, err
	}
	return report.ExportReport(plan, chartData)
}

func (p *Project) readKindArtifactPayload(kind ArtifactKind, id string) ([]byte, error) {
	if _, err := NewArtifactKind(string(kind)); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("artifact id is required")
	}

	manifestPath := ArtifactManifestPath(p.Manifest.ArtifactRoot, kind, id)
	raw, err := p.ReadArtifact(manifestPath)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
