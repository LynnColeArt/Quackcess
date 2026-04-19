package project

import "fmt"

const schemaVersion = "1.0.0"

const (
	defaultFormat       = "quackcess.qdb"
	defaultDataFile     = "database.duckdb"
	defaultArtifactRoot = "artifacts/"
)

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

func DefaultManifest() Manifest {
	return Manifest{
		Format:       defaultFormat,
		Version:      CurrentSchemaVersion(),
		DataFile:     defaultDataFile,
		ArtifactRoot: defaultArtifactRoot,
	}
}

// MigrateManifest returns a stable manifest for the current schema version.
// The current implementation only supports migration from the current version.
func MigrateManifest(m Manifest) (Manifest, error) {
	if m.Version == "" {
		m.Version = CurrentSchemaVersion()
	}

	if m.Version == CurrentSchemaVersion() {
		if m.Format == "" {
			m.Format = defaultFormat
		}
		if m.DataFile == "" {
			m.DataFile = defaultDataFile
		}
		if m.ArtifactRoot == "" {
			m.ArtifactRoot = defaultArtifactRoot
		}
		return m, nil
	}

	return Manifest{}, fmt.Errorf("unsupported manifest version: %s", m.Version)
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
	if m.CreatedBy == "" {
		return fmt.Errorf("createdBy is required")
	}
	if m.DataFile == "" {
		return fmt.Errorf("dataFile is required")
	}
	if m.ArtifactRoot == "" {
		return fmt.Errorf("artifactRoot is required")
	}
	return nil
}
