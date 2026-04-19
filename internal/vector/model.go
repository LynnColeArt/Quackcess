package vector

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const currentVectorFieldSchemaVersion = "1.0.0"

type VectorField struct {
	ID                  string    `json:"id"`
	SchemaVersion       string    `json:"schemaVersion"`
	TableName           string    `json:"tableName"`
	SourceColumn        string    `json:"sourceColumn"`
	VectorColumn        string    `json:"vectorColumn"`
	Dimension           int       `json:"dimension"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	StaleAfterHours     int       `json:"staleAfterHours"`
	LastIndexedAt       time.Time `json:"lastIndexedAt"`
	SourceLastUpdatedAt time.Time `json:"sourceLastUpdatedAt"`
}

func CurrentVectorFieldSchemaVersion() string {
	return currentVectorFieldSchemaVersion
}

func ParseVectorField(raw []byte) (VectorField, error) {
	var field VectorField
	if err := json.Unmarshal(raw, &field); err != nil {
		return VectorField{}, fmt.Errorf("invalid vector field JSON: %w", err)
	}
	return CanonicalizeVectorField(field)
}

func MarshalVectorField(field VectorField) ([]byte, error) {
	canonical, err := CanonicalizeVectorField(field)
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

func CanonicalizeVectorField(field VectorField) (VectorField, error) {
	field.ID = strings.TrimSpace(field.ID)
	field.SchemaVersion = strings.TrimSpace(field.SchemaVersion)
	field.TableName = strings.TrimSpace(field.TableName)
	field.SourceColumn = strings.TrimSpace(field.SourceColumn)
	field.VectorColumn = strings.TrimSpace(field.VectorColumn)
	field.Provider = strings.TrimSpace(field.Provider)
	field.Model = strings.TrimSpace(field.Model)

	if field.SchemaVersion == "" {
		field.SchemaVersion = CurrentVectorFieldSchemaVersion()
	}
	if field.SchemaVersion != CurrentVectorFieldSchemaVersion() {
		return VectorField{}, fmt.Errorf("unsupported vector field schema version: %s", field.SchemaVersion)
	}
	if field.ID == "" {
		return VectorField{}, fmt.Errorf("vector field id is required")
	}
	if field.TableName == "" {
		return VectorField{}, fmt.Errorf("vector field tableName is required")
	}
	if field.SourceColumn == "" {
		return VectorField{}, fmt.Errorf("vector field sourceColumn is required")
	}
	if field.VectorColumn == "" {
		field.VectorColumn = defaultVectorColumnName(field.TableName, field.SourceColumn)
	}
	if field.Dimension <= 0 {
		return VectorField{}, fmt.Errorf("vector field dimension must be positive")
	}
	if field.Provider == "" {
		return VectorField{}, fmt.Errorf("vector field provider is required")
	}
	if field.Model == "" {
		return VectorField{}, fmt.Errorf("vector field model is required")
	}
	if field.StaleAfterHours < 0 {
		return VectorField{}, fmt.Errorf("vector field staleAfterHours cannot be negative")
	}
	if field.StaleAfterHours == 0 {
		field.StaleAfterHours = 24
	}

	return field, nil
}

func defaultVectorColumnName(tableName, sourceColumn string) string {
	return strings.TrimSpace(tableName + "_" + sourceColumn + "_vec")
}
