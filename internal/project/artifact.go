package project

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	artifactManifestFile = "manifest.json"
)

const (
	artifactSpecVersionV1 = "1.0.0"
	artifactSpecVersionV0 = "0.9.0"
)

type ArtifactKind string

const (
	ArtifactKindCanvas    ArtifactKind = "canvas"
	ArtifactKindQuery     ArtifactKind = "query"
	ArtifactKindChart     ArtifactKind = "chart"
	ArtifactKindReport    ArtifactKind = "report"
	ArtifactKindProcedure ArtifactKind = "procedure"
	ArtifactKindVectorOp  ArtifactKind = "vector_operation"
	ArtifactKindUnknown   ArtifactKind = "unknown"
)

var allArtifactKinds = map[ArtifactKind]struct{}{
	ArtifactKindCanvas:    {},
	ArtifactKindQuery:     {},
	ArtifactKindChart:     {},
	ArtifactKindReport:    {},
	ArtifactKindProcedure: {},
	ArtifactKindVectorOp:  {},
}

// ArtifactManifestPath returns where a versioned artifact manifest should live inside the qdb payload.
func ArtifactManifestPath(artifactRoot string, kind ArtifactKind, id string) string {
	return path.Clean(path.Join(artifactRoot, string(kind), id, artifactManifestFile))
}

// NewArtifactKind validates and normalizes an artifact kind string.
func NewArtifactKind(raw string) (ArtifactKind, error) {
	kind := ArtifactKind(strings.TrimSpace(raw))
	if kind == "" {
		return "", fmt.Errorf("artifact kind is required")
	}
	if kind == ArtifactKindUnknown {
		return "", fmt.Errorf("unsupported artifact kind: %s", raw)
	}
	if _, ok := allArtifactKinds[kind]; !ok {
		return "", fmt.Errorf("unsupported artifact kind: %s", raw)
	}
	return kind, nil
}

// ArtifactManifestPathValidated returns a manifest path after validating the kind and artifact id.
func ArtifactManifestPathValidated(kind ArtifactKind, id string) (string, error) {
	if _, err := NewArtifactKind(string(kind)); err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("artifact id is required")
	}
	return path.Clean(path.Join(string(kind), id, artifactManifestFile)), nil
}

// ArtifactSpecV1 is the canonical per-artifact metadata shape for phase 1 artifacts.
type ArtifactSpecV1 struct {
	ID            string       `json:"id"`
	Kind          ArtifactKind `json:"kind"`
	SchemaVersion string       `json:"schemaVersion"`
	Title         string       `json:"title,omitempty"`
	Description   string       `json:"description,omitempty"`
}

type VectorOperationSpec struct {
	ArtifactSpecV1
	SourceTable  string `json:"sourceTable"`
	SourceColumn string `json:"sourceColumn"`
	TargetColumn string `json:"targetColumn"`
	Filter       string `json:"filter,omitempty"`
	FieldID      string `json:"fieldId"`
	Built        bool   `json:"built"`
	BatchSize    int    `json:"batchSize"`
	VectorCount  int    `json:"vectorCount"`
	SkipReason   string `json:"skipReason,omitempty"`
	ExecutedBy    string `json:"executedBy,omitempty"`
	ExecutedAt    string `json:"executedAt"`
	CommandText   string `json:"commandText"`
}

type artifactSpecLegacyV0 struct {
	ID           string `json:"id"`
	ArtifactType string `json:"artifactType"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
}

type ArtifactIndexEntry struct {
	Path string
	ArtifactSpecV1
}

// CurrentArtifactSchemaVersion is the canonical artifact schema version for package contracts.
func CurrentArtifactSchemaVersion() string {
	return artifactSpecVersionV1
}

func ValidateArtifactSpecV1(spec ArtifactSpecV1) error {
	_, err := NewArtifactKind(string(spec.Kind))
	if err != nil {
		return err
	}
	if strings.TrimSpace(spec.ID) == "" {
		return fmt.Errorf("artifact id is required")
	}
	if strings.TrimSpace(spec.SchemaVersion) == "" {
		return fmt.Errorf("artifact schema version is required")
	}
	return nil
}

// ParseArtifactSpec parses a raw artifact manifest and migrates known historical versions.
func ParseArtifactSpec(raw []byte) (ArtifactSpecV1, error) {
	var probe map[string]interface{}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ArtifactSpecV1{}, fmt.Errorf("invalid artifact spec JSON: %w", err)
	}

	schemaRaw, ok := probe["schemaVersion"]
	if !ok {
		schemaRaw = artifactSpecVersionV0
	}
	schemaVersion, ok := schemaRaw.(string)
	if !ok {
		return ArtifactSpecV1{}, fmt.Errorf("schemaVersion must be string")
	}
	schemaVersion = strings.TrimSpace(schemaVersion)

	switch schemaVersion {
	case "", artifactSpecVersionV0:
		var legacy artifactSpecLegacyV0
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return ArtifactSpecV1{}, fmt.Errorf("invalid legacy artifact spec: %w", err)
		}
		kind, err := NewArtifactKind(legacy.ArtifactType)
		if err != nil {
			return ArtifactSpecV1{}, err
		}
		migrated := ArtifactSpecV1{
			ID:            legacy.ID,
			Kind:          kind,
			SchemaVersion: CurrentArtifactSchemaVersion(),
			Title:         legacy.Title,
			Description:   legacy.Description,
		}
		if err := ValidateArtifactSpecV1(migrated); err != nil {
			return ArtifactSpecV1{}, err
		}
		return migrated, nil
	case artifactSpecVersionV1:
		var spec ArtifactSpecV1
		if err := json.Unmarshal(raw, &spec); err != nil {
			return ArtifactSpecV1{}, fmt.Errorf("invalid artifact spec: %w", err)
		}
		if err := ValidateArtifactSpecV1(spec); err != nil {
			return ArtifactSpecV1{}, err
		}
		return spec, nil
	default:
		return ArtifactSpecV1{}, fmt.Errorf("unsupported artifact schema version: %s", schemaVersion)
	}
}

func ParseVectorOperationArtifactSpec(raw []byte) (VectorOperationSpec, error) {
	var spec VectorOperationSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return VectorOperationSpec{}, fmt.Errorf("invalid vector operation artifact spec: %w", err)
	}
	if err := ValidateArtifactSpecV1(spec.ArtifactSpecV1); err != nil {
		return VectorOperationSpec{}, err
	}
	if spec.Kind != ArtifactKindVectorOp {
		return VectorOperationSpec{}, fmt.Errorf("artifact kind mismatch for vector operation: %s", spec.Kind)
	}
	if strings.TrimSpace(spec.SourceTable) == "" {
		return VectorOperationSpec{}, fmt.Errorf("vector operation sourceTable is required")
	}
	if strings.TrimSpace(spec.SourceColumn) == "" {
		return VectorOperationSpec{}, fmt.Errorf("vector operation sourceColumn is required")
	}
	if strings.TrimSpace(spec.TargetColumn) == "" {
		return VectorOperationSpec{}, fmt.Errorf("vector operation targetColumn is required")
	}
	if strings.TrimSpace(spec.FieldID) == "" {
		return VectorOperationSpec{}, fmt.Errorf("vector operation fieldId is required")
	}
	if strings.TrimSpace(spec.CommandText) == "" {
		return VectorOperationSpec{}, fmt.Errorf("vector operation commandText is required")
	}
	if strings.TrimSpace(spec.ExecutedAt) == "" {
		return VectorOperationSpec{}, fmt.Errorf("vector operation executedAt is required")
	}
	if spec.BatchSize < 0 {
		return VectorOperationSpec{}, fmt.Errorf("vector operation batchSize must be non-negative")
	}
	if spec.VectorCount < 0 {
		return VectorOperationSpec{}, fmt.Errorf("vector operation vectorCount must be non-negative")
	}
	return spec, nil
}

func MarshalVectorOperationSpec(spec VectorOperationSpec) ([]byte, error) {
	spec, err := canonicalizeVectorOperationSpec(spec)
	if err != nil {
		return nil, err
	}
	return json.Marshal(spec)
}

func canonicalizeVectorOperationSpec(spec VectorOperationSpec) (VectorOperationSpec, error) {
	spec.ID = strings.TrimSpace(spec.ID)
	spec.Kind = ArtifactKindVectorOp
	spec.SchemaVersion = strings.TrimSpace(spec.SchemaVersion)
	if spec.SchemaVersion == "" {
		spec.SchemaVersion = CurrentArtifactSchemaVersion()
	}
	spec.SourceTable = strings.TrimSpace(spec.SourceTable)
	spec.SourceColumn = strings.TrimSpace(spec.SourceColumn)
	spec.TargetColumn = strings.TrimSpace(spec.TargetColumn)
	spec.Filter = strings.TrimSpace(spec.Filter)
	spec.FieldID = strings.TrimSpace(spec.FieldID)
	spec.ExecutedBy = strings.TrimSpace(spec.ExecutedBy)
	spec.ExecutedAt = strings.TrimSpace(spec.ExecutedAt)
	spec.CommandText = strings.TrimSpace(spec.CommandText)
	spec.SkipReason = strings.TrimSpace(spec.SkipReason)
	return spec, nil
}

// BuildArtifactIndex returns a deterministic map of canonical artifact paths to specs.
// It validates each spec, rejects duplicate IDs, and rejects path/spec mismatches (stale manifests).
func BuildArtifactIndex(artifactRoot string, artifacts map[string][]byte) (map[string]ArtifactIndexEntry, error) {
	if artifactRoot == "" {
		artifactRoot = defaultArtifactRoot
	}

	artifactNames := make([]string, 0, len(artifacts))
	for path := range artifacts {
		artifactNames = append(artifactNames, path)
	}
	sort.Strings(artifactNames)

	seenIDs := map[string]string{}
	index := map[string]ArtifactIndexEntry{}
	for _, artifactPath := range artifactNames {
		canonical, err := normalizedArtifactPath(artifactRoot, artifactPath)
		if err != nil {
			return nil, err
		}
		spec, err := ParseArtifactSpec(artifacts[artifactPath])
		if err != nil {
			return nil, fmt.Errorf("invalid artifact spec for %s: %w", canonical, err)
		}
		expectedPath := ArtifactManifestPath(artifactRoot, spec.Kind, spec.ID)
		if expectedPath != canonical {
			return nil, fmt.Errorf("artifact manifest path mismatch for id %s: %s", spec.ID, canonical)
		}
		if _, exists := index[canonical]; exists {
			return nil, fmt.Errorf("artifact path collision: %s", canonical)
		}
		if existing, ok := seenIDs[spec.ID]; ok {
			return nil, fmt.Errorf("duplicate artifact id: %s (%s, %s)", spec.ID, existing, canonical)
		}
		seenIDs[spec.ID] = canonical
		index[canonical] = ArtifactIndexEntry{
			Path:           canonical,
			ArtifactSpecV1: spec,
		}
	}
	return index, nil
}
