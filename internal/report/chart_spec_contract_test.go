package report

import (
	"encoding/json"
	"testing"
)

func TestChartSpecParseCanonicalizesDefaults(t *testing.T) {
	raw := []byte(`{
		"id":" chart-1 ",
		"kind":"chart",
		"renderer":"Mermaid",
		"chartType":"bar",
		"sourceType":"QUERY",
		"source":" SELECT 1 "
	}`)

	spec, err := ParseChartSpec(raw)
	if err != nil {
		t.Fatalf("parse chart spec: %v", err)
	}
	if got, want := spec.ID, "chart-1"; got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}
	if got, want := spec.Kind, "chart"; got != want {
		t.Fatalf("kind = %q, want %q", got, want)
	}
	if got, want := spec.SchemaVersion, CurrentChartSchemaVersion(); got != want {
		t.Fatalf("schemaVersion = %q, want %q", got, want)
	}
	if got, want := spec.Renderer, ChartRendererMermaid; got != want {
		t.Fatalf("renderer = %q, want %q", got, want)
	}
	if got, want := spec.SourceType, ChartSourceQuery; got != want {
		t.Fatalf("sourceType = %q, want %q", got, want)
	}
}

func TestChartSpecParseRejectsInvalidJSON(t *testing.T) {
	_, err := ParseChartSpec([]byte(`{bad json`))
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestChartSpecRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing id",
			raw:  `{"kind":"chart","renderer":"mermaid","chartType":"line","sourceType":"query","source":"SELECT 1"}`,
		},
		{
			name: "unsupported kind",
			raw:  `{"id":"c","kind":"query","renderer":"mermaid","chartType":"line","sourceType":"query","source":"SELECT 1"}`,
		},
		{
			name: "unsupported renderer",
			raw:  `{"id":"c","kind":"chart","renderer":"highcharts","chartType":"line","sourceType":"query","source":"SELECT 1"}`,
		},
		{
			name: "missing chart type",
			raw:  `{"id":"c","kind":"chart","renderer":"mermaid","chartType":"  ","sourceType":"query","source":"SELECT 1"}`,
		},
		{
			name: "unsupported source type",
			raw:  `{"id":"c","kind":"chart","renderer":"mermaid","chartType":"bar","sourceType":"rest","source":"SELECT 1"}`,
		},
		{
			name: "missing source",
			raw:  `{"id":"c","kind":"chart","renderer":"mermaid","chartType":"bar","sourceType":"query","source":"   "}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseChartSpec([]byte(tc.raw))
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestChartSpecMarshalProducesCanonicalJSON(t *testing.T) {
	in := ChartSpec{
		ID:         "sales-over-time",
		Kind:       "chart",
		Renderer:   "Mermaid",
		ChartType:  "line",
		SourceType: "sql",
		Source:     "SELECT * FROM events",
	}
	in.SchemaVersion = "" // intentionally omitted

	out, err := MarshalChartSpec(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ChartSpec
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if got.ID != in.ID {
		t.Fatalf("marshaled id mismatch: %q", got.ID)
	}
	if got.SchemaVersion != CurrentChartSchemaVersion() {
		t.Fatalf("marshaled schemaVersion mismatch: %q", got.SchemaVersion)
	}
}
