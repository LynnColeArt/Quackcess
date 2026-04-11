package project

import "fmt"

const schemaVersion = "1.0.0"

// Manifest describes a .qdb project package.
type Manifest struct {
	Format       string `json:"format"`
	Version      string `json:"version"`
	ProjectName  string `json:"projectName"`
	CreatedBy    string `json:"createdBy"`
	DataFile     string `json:"dataFile"`
	ArtifactRoot string `json:"artifactRoot"`
}

func CurrentSchemaVersion() string {
	return schemaVersion
}

func ValidateManifest(m Manifest) error {
	if m.Format == "" {
		return fmt.Errorf("format is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if m.Version != schemaVersion {
		return fmt.Errorf("unsupported manifest version: %s", m.Version)
	}
	if m.ProjectName == "" {
		return fmt.Errorf("projectName is required")
	}
	if m.DataFile == "" {
		return fmt.Errorf("dataFile is required")
	}
	return nil
}
