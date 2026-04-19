package project

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

const (
	manifestEntry = "manifest.json"
)

type CreateOptions struct {
	Manifest           Manifest
	DatabaseSourcePath string
	Artifacts          map[string][]byte
}

type Project struct {
	Path     string
	Manifest Manifest
}

// Create writes a .qdb archive containing manifest, duckdb payload, and optional artifacts.
func Create(path string, options CreateOptions) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("project path is required")
	}

	manifest := options.Manifest
	if manifest.ProjectName == "" {
		manifest.ProjectName = deriveProjectName(path)
	}
	if manifest.CreatedBy == "" {
		manifest.CreatedBy = "quackcess-user"
	}

	migrated, err := MigrateManifest(manifest)
	if err != nil {
		return err
	}
	if err := ValidateManifest(migrated); err != nil {
		return err
	}

	manifest = migrated
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	dbFile := options.DatabaseSourcePath
	var dbBytes []byte
	if dbFile != "" {
		dbBytes, err = os.ReadFile(dbFile)
		if err != nil {
			return fmt.Errorf("read database source: %w", err)
		}
	} else {
		dbBytes, err = createEmptyDuckDBBytes()
		if err != nil {
			return fmt.Errorf("initialize embedded database: %w", err)
		}
	}

	index, err := BuildArtifactIndex(manifest.ArtifactRoot, options.Artifacts)
	if err != nil {
		return err
	}

	// Deterministic write order.
	artifactOutputPaths := make([]string, 0, len(index))
	for path := range index {
		artifactOutputPaths = append(artifactOutputPaths, path)
	}
	sort.Strings(artifactOutputPaths)

	return writeZipAtomically(path, func(zw *zip.Writer) error {
		if err := writeZipEntry(zw, manifestEntry, bytes.NewReader(manifestData)); err != nil {
			return err
		}
		if err := writeZipEntry(zw, manifest.DataFile, bytes.NewReader(dbBytes)); err != nil {
			return err
		}
		for _, artifactPath := range artifactOutputPaths {
			payload, ok := options.Artifacts[artifactPath]
			if !ok {
				for inputPath, artifactPayload := range options.Artifacts {
					canonicalPath, err := normalizedArtifactPath(manifest.ArtifactRoot, inputPath)
					if err != nil {
						return err
					}
					if canonicalPath == artifactPath {
						payload = artifactPayload
						ok = true
						break
					}
				}
			}
			if !ok {
				return fmt.Errorf("artifact payload missing for %s", artifactPath)
			}
			if err := writeArtifactEntry(zw, artifactPath, payload); err != nil {
				return err
			}
		}
		return nil
	})
}

func createEmptyDuckDBBytes() ([]byte, error) {
	tmp, err := os.CreateTemp("", "quackcess-project-*.duckdb")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return nil, err
	}
	if err := os.Remove(tmpName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	defer func() {
		_ = os.Remove(tmpName)
	}()

	sqlDB, err := sql.Open("duckdb", tmpName)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := sqlDB.Close(); err != nil {
		return nil, err
	}

	return os.ReadFile(tmpName)
}

// Open reads a .qdb archive into Project metadata.
func Open(path string) (*Project, error) {
	manifest, err := readManifestFromArchive(path)
	if err != nil {
		return nil, err
	}

	migrated, err := MigrateManifest(manifest)
	if err != nil {
		return nil, err
	}
	if err := ValidateManifest(migrated); err != nil {
		return nil, err
	}
	if err := validateProjectArchiveContents(path, migrated); err != nil {
		return nil, err
	}

	return &Project{Path: path, Manifest: migrated}, nil
}

func readManifestFromArchive(path string) (Manifest, error) {
	zfile, err := zip.OpenReader(path)
	if err != nil {
		return Manifest{}, err
	}
	defer zfile.Close()

	for _, f := range zfile.File {
		if f.Name != manifestEntry {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			return Manifest{}, err
		}
		defer reader.Close()

		var manifest Manifest
		if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
			return Manifest{}, err
		}
		return manifest, nil
	}

	return Manifest{}, fmt.Errorf("manifest entry missing")
}

// ListContents lists all archive entries, sorted for deterministic output.
func ListContents(path string) ([]string, error) {
	zfile, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zfile.Close()

	names := make([]string, 0, len(zfile.File))
	for _, f := range zfile.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (p *Project) Contents() ([]string, error) {
	return ListContents(p.Path)
}

// ReadDataFile returns the embedded data file bytes from the project archive.
func (p *Project) ReadDataFile() ([]byte, error) {
	return p.ReadArtifact(p.Manifest.DataFile)
}

func (p *Project) ReadArtifact(name string) ([]byte, error) {
	zfile, err := zip.OpenReader(p.Path)
	if err != nil {
		return nil, err
	}
	defer zfile.Close()

	for _, f := range zfile.File {
		if f.Name != name {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}

	return nil, errors.New("artifact not found")
}

func (p *Project) UpsertArtifact(raw []byte) error {
	if p == nil {
		return fmt.Errorf("project is required")
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("project path is required")
	}
	if len(raw) == 0 {
		return fmt.Errorf("artifact payload is required")
	}
	spec, err := ParseArtifactSpec(raw)
	if err != nil {
		return err
	}
	manifestPath := ArtifactManifestPath(p.Manifest.ArtifactRoot, spec.Kind, spec.ID)
	if err := validateArtifactPath(p.Manifest.ArtifactRoot, manifestPath); err != nil {
		return err
	}

	src, err := zip.OpenReader(p.Path)
	if err != nil {
		return err
	}
	defer src.Close()

	return writeZipAtomically(p.Path, func(zw *zip.Writer) error {
		replaced := false
		for _, f := range src.File {
			reader, err := f.Open()
			if err != nil {
				return err
			}
			payload, readErr := io.ReadAll(reader)
			_ = reader.Close()
			if readErr != nil {
				return readErr
			}
			entryPath := f.Name
			if entryPath == manifestPath {
				if err := writeArtifactEntry(zw, entryPath, raw); err != nil {
					return err
				}
				replaced = true
				continue
			}
			if err := writeZipEntry(zw, entryPath, bytes.NewReader(payload)); err != nil {
				return err
			}
		}
		if !replaced {
			if err := writeArtifactEntry(zw, manifestPath, raw); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateArtifactPath(artifactRoot, artifactPath string) error {
	cleanPath, err := normalizedArtifactPath(artifactRoot, artifactPath)
	if err != nil {
		return err
	}
	if cleanPath == "" {
		return fmt.Errorf("artifact path is required")
	}
	return nil
}

func validateProjectArchiveContents(path string, manifest Manifest) error {
	zfile, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zfile.Close()

	found := false
	for _, f := range zfile.File {
		if f.Name == manifest.DataFile {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("required data file missing: %s", manifest.DataFile)
	}
	return nil
}

func normalizedArtifactPath(artifactRoot, artifactPath string) (string, error) {
	artifactPath = strings.TrimSpace(artifactPath)
	artifactRoot = strings.TrimSpace(artifactRoot)
	if artifactPath == "" {
		return "", fmt.Errorf("artifact path is required")
	}
	if strings.Contains(artifactPath, "..") || filepath.IsAbs(artifactPath) {
		return "", fmt.Errorf("invalid artifact path: %s", artifactPath)
	}
	if artifactRoot == "" {
		artifactRoot = defaultArtifactRoot
	}
	if !strings.HasSuffix(artifactRoot, "/") {
		artifactRoot += "/"
	}
	cleanRoot := path.Clean(artifactRoot)
	cleanPath := path.Clean(artifactPath)
	if cleanRoot == "." {
		cleanRoot = ""
	}
	if cleanRoot != "" && !strings.HasPrefix(cleanPath+"/", cleanRoot) && cleanPath != strings.TrimSuffix(cleanRoot, "/") {
		return "", fmt.Errorf("invalid artifact path: %s", artifactPath)
	}
	if cleanPath == strings.TrimSuffix(cleanRoot, "/") {
		return "", fmt.Errorf("invalid artifact path: %s", artifactPath)
	}
	return cleanPath, nil
}

func writeArtifactEntry(zw *zip.Writer, artifactPath string, data []byte) error {
	return writeZipEntry(zw, artifactPath, bytes.NewReader(data))
}

func writeZipAtomically(path string, writeBody func(zw *zip.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	zw := zip.NewWriter(tmp)
	if err := writeBody(zw); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func writeZipEntry(zw *zip.Writer, path string, data *bytes.Reader) error {
	entry, err := zw.Create(path)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, data)
	return err
}

func deriveProjectName(path string) string {
	base := filepath.Base(path)
	if base == "" || base == "." {
		return "project"
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}
