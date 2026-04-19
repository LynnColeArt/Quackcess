package report

import (
	"testing"
)

func TestReportSpecParseCanonicalizesDefaultsAndSections(t *testing.T) {
	raw := []byte(`{
		"id":" report-1 ",
		"kind":"report",
		"sections":[
			{"id":" a ","kind":"text","title":" Intro ","text":" hello "},
			{"id":" b ","kind":"chart","title":"Graph","chartId":" c1 "}
		]
	}`)

	reports, err := ParseReportSpec(raw)
	if err != nil {
		t.Fatalf("parse report: %v", err)
	}
	if got, want := reports.ID, "report-1"; got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}
	if got, want := reports.Kind, "report"; got != want {
		t.Fatalf("kind = %q, want %q", got, want)
	}
	if got, want := reports.SchemaVersion, CurrentReportSchemaVersion(); got != want {
		t.Fatalf("schemaVersion = %q, want %q", got, want)
	}
	if got, want := reports.Sections[0].ID, "a"; got != want {
		t.Fatalf("section id = %q, want %q", got, want)
	}
	if got, want := reports.Sections[0].Text, "hello"; got != want {
		t.Fatalf("section text = %q, want %q", got, want)
	}
	if got, want := reports.Sections[1].ChartID, "c1"; got != want {
		t.Fatalf("section chartId = %q, want %q", got, want)
	}
}

func TestReportSpecCanonicalizesNilSectionsToEmptySlice(t *testing.T) {
	raw := []byte(`{"id":"r-empty-sections","kind":"report","sections":null}`)

	spec, err := ParseReportSpec(raw)
	if err != nil {
		t.Fatalf("parse report: %v", err)
	}
	if spec.Sections == nil {
		t.Fatal("sections should be normalized to empty slice")
	}
	if len(spec.Sections) != 0 {
		t.Fatalf("expected 0 sections, got %d", len(spec.Sections))
	}
}

func TestReportSpecRequiresTextOrChartLinkForKnownKinds(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing id",
			raw:  `{"kind":"report","sections":[{"id":"s1","kind":"text","text":"ok"}]}`,
		},
		{
			name: "text section missing content",
			raw:  `{"id":"r1","kind":"report","sections":[{"id":"s1","kind":"text","text":"   "}]}`,
		},
		{
			name: "chart section missing chart id",
			raw:  `{"id":"r1","kind":"report","sections":[{"id":"s1","kind":"chart","chartId":"   "}]}`,
		},
		{
			name: "unsupported section kind",
			raw:  `{"id":"r1","kind":"report","sections":[{"id":"s1","kind":"table","chartId":"c1"}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseReportSpec([]byte(tc.raw))
			if err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestReportSpecDetectsDuplicateSectionIDs(t *testing.T) {
	raw := []byte(`{
		"id":"dup-sections",
		"kind":"report",
		"sections":[
			{"id":"same","kind":"text","text":"a"},
			{"id":"same","kind":"chart","chartId":"c1"}
		]
	}`)

	_, err := ParseReportSpec(raw)
	if err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestReportSpecParseRejectsInvalidJSON(t *testing.T) {
	_, err := ParseReportSpec([]byte(`{broken`))
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
