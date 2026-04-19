package vector

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseVectorFieldCanonicalizesDefaults(t *testing.T) {
	raw := []byte(`{
		"id":" vf-1 ",
		"tableName":" documents ",
		"sourceColumn":" content ",
		"dimension":768,
		"provider":"qwen-local",
		"model":"qwen3.5-0.8b",
		"staleAfterHours":0
	}`)

	field, err := ParseVectorField(raw)
	if err != nil {
		t.Fatalf("parse vector field: %v", err)
	}
	if got, want := field.ID, "vf-1"; got != want {
		t.Fatalf("id = %q, want %q", got, want)
	}
	if got, want := field.VectorColumn, "documents_content_vec"; got != want {
		t.Fatalf("vectorColumn = %q, want %q", got, want)
	}
	if got, want := field.SchemaVersion, CurrentVectorFieldSchemaVersion(); got != want {
		t.Fatalf("schemaVersion = %q, want %q", got, want)
	}
	if field.StaleAfterHours != 24 {
		t.Fatalf("staleAfterHours = %d, want %d", field.StaleAfterHours, 24)
	}
}

func TestParseVectorFieldRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing id",
			raw:  `{"tableName":"docs","sourceColumn":"content","dimension":768,"provider":"qwen-local","model":"qwen3.5"}`,
		},
		{
			name: "missing table",
			raw:  `{"id":"vf","sourceColumn":"content","dimension":768,"provider":"qwen-local","model":"qwen3.5"}`,
		},
		{
			name: "missing sourceColumn",
			raw:  `{"id":"vf","tableName":"docs","dimension":768,"provider":"qwen-local","model":"qwen3.5"}`,
		},
		{
			name: "missing dimension",
			raw:  `{"id":"vf","tableName":"docs","sourceColumn":"content","provider":"qwen-local","model":"qwen3.5"}`,
		},
		{
			name: "bad schema version",
			raw:  `{"id":"vf","tableName":"docs","sourceColumn":"content","dimension":768,"provider":"qwen-local","model":"qwen3.5","schemaVersion":"0.0.1"}`,
		},
		{
			name: "negative stale policy",
			raw:  `{"id":"vf","tableName":"docs","sourceColumn":"content","dimension":768,"provider":"qwen-local","model":"qwen3.5","staleAfterHours":-1}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseVectorField([]byte(tc.raw)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestVectorFieldMarshalIsCanonical(t *testing.T) {
	field := VectorField{
		ID:              "vf-2",
		TableName:       "docs",
		SourceColumn:    "body",
		Dimension:       512,
		Provider:        "qwen-local",
		Model:           "qwen3.5-0.8b",
		StaleAfterHours: 12,
	}

	raw, err := MarshalVectorField(field)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got VectorField
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if got.SchemaVersion != CurrentVectorFieldSchemaVersion() {
		t.Fatalf("schemaVersion = %q, want %q", got.SchemaVersion, CurrentVectorFieldSchemaVersion())
	}
	if got.VectorColumn != "docs_body_vec" {
		t.Fatalf("vectorColumn = %q, want %q", got.VectorColumn, "docs_body_vec")
	}
	if !strings.Contains(string(raw), `"schemaVersion":"1.0.0"`) {
		t.Fatalf("output should contain version: %s", string(raw))
	}
}
