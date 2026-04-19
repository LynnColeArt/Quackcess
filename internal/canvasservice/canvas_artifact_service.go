package canvasservice

import (
	"fmt"
	"strings"

	"github.com/LynnColeArt/Quackcess/internal/catalog"
	"github.com/LynnColeArt/Quackcess/internal/query"
)

type CanvasArtifactHistory struct {
	ID        string
	Name      string
	Kind      string
	Version   int
	SourceRef string
	UpdatedAt string
}

type CanvasArtifactService struct {
	repository *catalog.CanvasRepository
}

func NewCanvasArtifactService(repository *catalog.CanvasRepository) *CanvasArtifactService {
	return &CanvasArtifactService{repository: repository}
}

func (s *CanvasArtifactService) CanvasRepository() *catalog.CanvasRepository {
	if s == nil {
		return nil
	}
	return s.repository
}

func (s *CanvasArtifactService) ListByKind(kind string) ([]catalog.Canvas, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("canvas service is not configured")
	}
	return s.repository.ListByKind(kind)
}

func (s *CanvasArtifactService) GetForExecution(name string) (query.CanvasSpec, error) {
	if s == nil || s.repository == nil {
		return query.CanvasSpec{}, fmt.Errorf("canvas service is not configured")
	}
	canvas, err := s.findByNameOrID(strings.TrimSpace(name))
	if err != nil {
		return query.CanvasSpec{}, err
	}
	spec, err := query.ParseCanvasSpec([]byte(canvas.SpecJSON))
	if err != nil {
		return query.CanvasSpec{}, err
	}
	return spec, nil
}

func (s *CanvasArtifactService) History(identifier string) ([]CanvasArtifactHistory, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("canvas service is not configured")
	}
	canvas, err := s.findByNameOrID(strings.TrimSpace(identifier))
	if err != nil {
		{
			return nil, err
		}
	}
	return []CanvasArtifactHistory{
		{
			ID:        canvas.ID,
			Name:      canvas.Name,
			Kind:      canvas.Kind,
			Version:   canvas.Version,
			SourceRef: canvas.SourceRef,
			UpdatedAt: canvas.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
	}, nil
}

func (s *CanvasArtifactService) CreateDraftCanvas(name string) (catalog.Canvas, error) {
	if s == nil || s.repository == nil {
		return catalog.Canvas{}, fmt.Errorf("canvas service is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return catalog.Canvas{}, fmt.Errorf("canvas name is required")
	}

	if _, err := s.repository.FindByName(name); err == nil {
		return catalog.Canvas{}, fmt.Errorf("canvas already exists: %s", name)
	}

	id := buildCanvasIDFromName(name)
	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		canvas := catalog.Canvas{
			ID:       id,
			Name:     name,
			Kind:     "query",
			SpecJSON: `{"nodes":[],"edges":[]}`,
			Version:  1,
		}
		if err := s.repository.Create(canvas); err != nil {
			if isDuplicateCanvasError(err) {
				id = buildCanvasIDWithSuffix(name, attempt+1)
				continue
			}
			return catalog.Canvas{}, err
		}
		created, err := s.repository.FindByName(name)
		if err != nil {
			return catalog.Canvas{}, err
		}
		return created, nil
	}
	return catalog.Canvas{}, fmt.Errorf("failed to allocate unique canvas id after %d attempts", maxAttempts)
}

func (s *CanvasArtifactService) RenameCanvas(oldName, newName string) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("canvas service is not configured")
	}
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("canvas rename requires old and new name")
	}
	canvas, err := s.repository.FindByName(oldName)
	if err != nil {
		return err
	}
	canvas.Name = newName
	canvas.Version = canvas.Version + 1
	return s.repository.Update(canvas)
}

func (s *CanvasArtifactService) DeleteCanvas(name string) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("canvas service is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("canvas name is required")
	}
	canvas, err := s.repository.FindByName(name)
	if err != nil {
		return err
	}
	return s.repository.Delete(canvas.ID)
}

func (s *CanvasArtifactService) SaveCanvasSpec(name, specJSON, sourceRef string) error {
	if s == nil || s.repository == nil {
		return fmt.Errorf("canvas service is not configured")
	}
	name = strings.TrimSpace(name)
	specJSON = strings.TrimSpace(specJSON)
	if name == "" || specJSON == "" {
		return fmt.Errorf("canvas name and spec json are required")
	}
	spec, err := query.ParseCanvasSpec([]byte(specJSON))
	if err != nil {
		return err
	}
	normalizedSpec, err := query.MarshalCanvasSpec(spec)
	if err != nil {
		return err
	}

	canvas, err := s.repository.FindByName(name)
	if err != nil {
		return err
	}
	canvas.Version = canvas.Version + 1
	canvas.SpecJSON = string(normalizedSpec)
	if strings.TrimSpace(sourceRef) != "" {
		canvas.SourceRef = strings.TrimSpace(sourceRef)
	}
	return s.repository.Update(canvas)
}

func (s *CanvasArtifactService) findByNameOrID(identifier string) (catalog.Canvas, error) {
	if identifier == "" {
		return catalog.Canvas{}, fmt.Errorf("canvas name is required")
	}
	if canvas, err := s.repository.GetByID(identifier); err == nil {
		return canvas, nil
	}
	return s.repository.FindByName(identifier)
}

func buildCanvasIDFromName(name string) string {
	slug := makeSlug(name)
	if slug == "" {
		return "canvas"
	}
	return "canvas-" + slug
}

func buildCanvasIDWithSuffix(name string, attempt int) string {
	slug := makeSlug(name)
	if slug == "" {
		return "canvas-" + fmt.Sprintf("%d", attempt)
	}
	return "canvas-" + slug + "-" + fmt.Sprintf("%d", attempt)
}

func makeSlug(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "canvas"
	}
	builder := strings.Builder{}
	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		isLetter := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		if isLetter || isDigit || ch == '-' || ch == '_' {
			builder.WriteByte(ch)
			continue
		}
		if ch == ' ' {
			builder.WriteByte('-')
		}
	}
	slug := strings.Trim(builder.String(), "-_")
	if slug == "" {
		return "canvas"
	}
	return slug
}

func isDuplicateCanvasError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "Duplicate key")
}
