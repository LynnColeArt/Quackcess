package query

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	canvasSpecVersion       = "1.0.0"
	canvasNodeMinSize       = 80.0
	canvasNodeDefaultWidth  = 240.0
	canvasNodeDefaultHeight = 140.0

	CanvasNodeKindTable = "table"
	CanvasEdgeKindJoin  = "join"
)

type CanvasSpec struct {
	Version string       `json:"version"`
	Name    string       `json:"name,omitempty"`
	Nodes   []CanvasNode `json:"nodes"`
	Edges   []CanvasEdge `json:"edges"`
}

type CanvasNode struct {
	ID     string        `json:"id"`
	Kind   string        `json:"kind"`
	Table  string        `json:"table"`
	Alias  string        `json:"alias"`
	X      float64       `json:"x"`
	Y      float64       `json:"y"`
	Width  float64       `json:"width"`
	Height float64       `json:"height"`
	Fields []CanvasField `json:"fields"`

	SelectedFields []string `json:"selected_fields"`
}

type CanvasField struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

type CanvasEdge struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	FromNode     string   `json:"from"`
	ToNode       string   `json:"to"`
	JoinType     string   `json:"joinType"`
	FromColumn   string   `json:"fromColumn"`
	ToColumn     string   `json:"toColumn"`
	Description  string   `json:"description,omitempty"`
	Label        string   `json:"label,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Cardinality  string   `json:"cardinality,omitempty"`
	DirectedHint bool     `json:"directed_hint"`
}

type CanvasSelection struct {
	ActiveNodeID    string   `json:"activeNodeID"`
	ActiveEdgeID    string   `json:"activeEdgeID"`
	SelectedNodeIDs []string `json:"selectedNodeIDs"`
	SelectedEdgeIDs []string `json:"selectedEdgeIDs"`
}

type CanvasDocument struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Version     string          `json:"version"`
	Spec        CanvasSpec      `json:"spec"`
	Selection   CanvasSelection `json:"selection"`
}

type CanvasPreview struct {
	DocumentID string
	SQLText    string
	Parameters []any
}

// ParseCanvasSpec validates and normalizes serialized canvas JSON.
func ParseCanvasSpec(raw []byte) (CanvasSpec, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return CanvasSpec{}, fmt.Errorf("canvas spec is required")
	}

	var spec CanvasSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return CanvasSpec{}, err
	}
	return NormalizeCanvasSpec(spec)
}

// ParseCanvasDocument validates and normalizes serialized canvas documents.
func ParseCanvasDocument(raw []byte) (CanvasDocument, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return CanvasDocument{}, fmt.Errorf("canvas document is required")
	}

	var document CanvasDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return CanvasDocument{}, err
	}
	return NormalizeCanvasDocument(document)
}

// MoveCanvasNode updates a node position in a canvas specification.
func MoveCanvasNode(spec CanvasSpec, nodeID string, x, y float64) (CanvasSpec, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return CanvasSpec{}, err
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return CanvasSpec{}, fmt.Errorf("node id is required")
	}

	moved := false
	for i, node := range normalized.Nodes {
		if node.ID != nodeID {
			continue
		}
		node.X = normalizeCanvasCoordinate(x)
		node.Y = normalizeCanvasCoordinate(y)
		normalized.Nodes[i] = node
		moved = true
		break
	}
	if !moved {
		return CanvasSpec{}, fmt.Errorf("node not found: %s", nodeID)
	}
	return NormalizeCanvasSpec(normalized)
}

// SetCanvasNodeSelectedFields updates selected fields for a node.
func SetCanvasNodeSelectedFields(spec CanvasSpec, nodeID string, fields []string) (CanvasSpec, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return CanvasSpec{}, err
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return CanvasSpec{}, fmt.Errorf("node id is required")
	}

	updated := false
	for i, node := range normalized.Nodes {
		if node.ID != nodeID {
			continue
		}
		node.SelectedFields = filterSelectedFields(normalizeSelectedFields(fields), node.Fields)
		normalized.Nodes[i] = node
		updated = true
		break
	}
	if !updated {
		return CanvasSpec{}, fmt.Errorf("node not found: %s", nodeID)
	}
	return NormalizeCanvasSpec(normalized)
}

// AddCanvasNode appends a normalized canvas node to a valid canvas specification.
func AddCanvasNode(spec CanvasSpec, node CanvasNode) (CanvasSpec, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return CanvasSpec{}, err
	}

	node = normalizeCanvasNode(node)
	if node.ID == "" {
		return CanvasSpec{}, fmt.Errorf("node id is required")
	}
	for _, existing := range normalized.Nodes {
		if strings.EqualFold(existing.ID, node.ID) {
			return CanvasSpec{}, fmt.Errorf("node id already exists: %s", node.ID)
		}
	}

	normalized.Nodes = append(normalized.Nodes, node)
	return NormalizeCanvasSpec(normalized)
}

// AddCanvasEdge appends a new edge to a normalized canvas spec.
func AddCanvasEdge(spec CanvasSpec, edge CanvasEdge) (CanvasSpec, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return CanvasSpec{}, err
	}
	edge = normalizeCanvasEdge(edge)
	if edge.ID == "" {
		return CanvasSpec{}, fmt.Errorf("edge id is required")
	}
	for _, existing := range normalized.Edges {
		if strings.EqualFold(existing.ID, edge.ID) {
			return CanvasSpec{}, fmt.Errorf("edge id already exists: %s", edge.ID)
		}
	}
	normalized.Edges = append(normalized.Edges, edge)
	return NormalizeCanvasSpec(normalized)
}

// PatchCanvasEdge updates an existing edge definition.
func PatchCanvasEdge(spec CanvasSpec, edge CanvasEdge) (CanvasSpec, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return CanvasSpec{}, err
	}
	edge = normalizeCanvasEdge(edge)
	if edge.ID == "" {
		return CanvasSpec{}, fmt.Errorf("edge id is required")
	}
	updated := false
	for i, existing := range normalized.Edges {
		if existing.ID != edge.ID {
			continue
		}
		normalized.Edges[i] = edge
		updated = true
		break
	}
	if !updated {
		return CanvasSpec{}, fmt.Errorf("edge not found: %s", edge.ID)
	}
	return NormalizeCanvasSpec(normalized)
}

// DeleteCanvasEdge removes an edge by id from a canvas specification.
func DeleteCanvasEdge(spec CanvasSpec, edgeID string) (CanvasSpec, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return CanvasSpec{}, err
	}
	edgeID = strings.TrimSpace(edgeID)
	if edgeID == "" {
		return CanvasSpec{}, fmt.Errorf("edge id is required")
	}

	next := make([]CanvasEdge, 0, len(normalized.Edges))
	deleted := false
	for _, edge := range normalized.Edges {
		if strings.EqualFold(edge.ID, edgeID) {
			deleted = true
			continue
		}
		next = append(next, edge)
	}
	if !deleted {
		return CanvasSpec{}, fmt.Errorf("edge not found: %s", edgeID)
	}
	normalized.Edges = next
	return NormalizeCanvasSpec(normalized)
}

// MarshalCanvasSpec normalizes and serializes a canvas specification.
func MarshalCanvasSpec(spec CanvasSpec) ([]byte, error) {
	normalized, err := NormalizeCanvasSpec(spec)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// MarshalCanvasDocument normalizes and serializes a canvas document.
func MarshalCanvasDocument(document CanvasDocument) ([]byte, error) {
	normalized, err := NormalizeCanvasDocument(document)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

// PreviewCanvasDocument builds SQL preview state from a canvas document.
func PreviewCanvasDocument(document CanvasDocument) (CanvasPreview, error) {
	normalized, err := NormalizeCanvasDocument(document)
	if err != nil {
		return CanvasPreview{}, err
	}
	sqlText, err := GenerateSQLFromCanvas(normalized.Spec)
	if err != nil {
		return CanvasPreview{}, err
	}
	return CanvasPreview{
		DocumentID: normalized.ID,
		SQLText:    sqlText.SQL,
		Parameters: append([]any(nil), sqlText.Parameters...),
	}, nil
}

// NormalizeCanvasSpec trims fields, fills defaults, and validates the specification.
func NormalizeCanvasSpec(raw CanvasSpec) (CanvasSpec, error) {
	spec := raw
	spec.Version = strings.TrimSpace(spec.Version)
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Version == "" {
		spec.Version = canvasSpecVersion
	}

	if len(spec.Nodes) == 0 {
		return CanvasSpec{}, fmt.Errorf("canvas requires at least one node")
	}

	nodeByID := map[string]CanvasNode{}
	aliasByID := map[string]string{}
	for i := range spec.Nodes {
		node := spec.Nodes[i]
		node.ID = strings.TrimSpace(node.ID)
		node.Kind = strings.TrimSpace(strings.ToLower(node.Kind))
		node.Table = strings.TrimSpace(node.Table)
		node.Alias = strings.TrimSpace(node.Alias)
		node.X = normalizeCanvasCoordinate(node.X)
		node.Y = normalizeCanvasCoordinate(node.Y)
		node.Width = normalizeCanvasDimension(node.Width, canvasNodeDefaultWidth)
		node.Height = normalizeCanvasDimension(node.Height, canvasNodeDefaultHeight)
		node.SelectedFields = normalizeSelectedFields(node.SelectedFields)

		if node.ID == "" {
			return CanvasSpec{}, fmt.Errorf("node id is required")
		}
		if _, exists := nodeByID[node.ID]; exists {
			return CanvasSpec{}, fmt.Errorf("duplicate node id: %s", node.ID)
		}

		if node.Kind == "" {
			node.Kind = CanvasNodeKindTable
		}
		if node.Kind != CanvasNodeKindTable {
			return CanvasSpec{}, fmt.Errorf("unsupported node kind: %s", node.Kind)
		}
		if node.Table == "" {
			return CanvasSpec{}, fmt.Errorf("table node requires a table name")
		}
		if node.Alias == "" {
			node.Alias = node.Table
		}
		node.Fields = normalizeCanvasFields(node.Fields, node.Alias)
		node.SelectedFields = filterSelectedFields(node.SelectedFields, node.Fields)
		spec.Nodes[i] = node
		nodeByID[node.ID] = node
		aliasKey := strings.ToLower(node.Alias)
		if _, exists := aliasByID[aliasKey]; exists {
			return CanvasSpec{}, fmt.Errorf("duplicate node alias: %s", node.Alias)
		}
		aliasByID[aliasKey] = node.ID
	}

	edgeByID := map[string]struct{}{}
	for i := range spec.Edges {
		edge := spec.Edges[i]
		edge.ID = strings.TrimSpace(edge.ID)
		edge.Kind = strings.TrimSpace(strings.ToLower(edge.Kind))
		edge.FromNode = strings.TrimSpace(edge.FromNode)
		edge.ToNode = strings.TrimSpace(edge.ToNode)
		edge.FromColumn = strings.TrimSpace(edge.FromColumn)
		edge.ToColumn = strings.TrimSpace(edge.ToColumn)
		edge.JoinType = strings.TrimSpace(strings.ToUpper(edge.JoinType))

		if edge.Kind == "" {
			edge.Kind = CanvasEdgeKindJoin
		}
		if edge.ID != "" {
			if _, exists := edgeByID[edge.ID]; exists {
				return CanvasSpec{}, fmt.Errorf("duplicate edge id: %s", edge.ID)
			}
			edgeByID[edge.ID] = struct{}{}
		}
		if edge.Kind != CanvasEdgeKindJoin {
			return CanvasSpec{}, fmt.Errorf("unsupported edge kind: %s", edge.Kind)
		}
		if edge.FromNode == "" || edge.ToNode == "" {
			return CanvasSpec{}, fmt.Errorf("edge requires from and to node ids")
		}
		if _, exists := nodeByID[edge.FromNode]; !exists {
			return CanvasSpec{}, fmt.Errorf("edge references missing node: %s", edge.FromNode)
		}
		if _, exists := nodeByID[edge.ToNode]; !exists {
			return CanvasSpec{}, fmt.Errorf("edge references missing node: %s", edge.ToNode)
		}
		if edge.FromColumn == "" || edge.ToColumn == "" {
			return CanvasSpec{}, fmt.Errorf("join edge requires from/to columns")
		}
		if _, err := ParseJoinType(edge.JoinType); err != nil {
			return CanvasSpec{}, err
		}
		if !canvasNodeHasField(nodeByID[edge.FromNode], edge.FromColumn) {
			return CanvasSpec{}, fmt.Errorf("edge references missing from column: %s", edge.FromColumn)
		}
		if !canvasNodeHasField(nodeByID[edge.ToNode], edge.ToColumn) {
			return CanvasSpec{}, fmt.Errorf("edge references missing to column: %s", edge.ToColumn)
		}
		spec.Edges[i] = edge
	}

	return spec, nil
}

// NormalizeCanvasDocument normalizes metadata, document state, and underlying spec.
func NormalizeCanvasDocument(document CanvasDocument) (CanvasDocument, error) {
	document.ID = strings.TrimSpace(document.ID)
	document.Name = strings.TrimSpace(document.Name)
	document.Kind = strings.TrimSpace(document.Kind)
	document.Title = strings.TrimSpace(document.Title)
	document.Description = strings.TrimSpace(document.Description)
	document.Version = strings.TrimSpace(document.Version)
	document.Tags = normalizeCanvasDocumentTags(document.Tags)

	if document.Version == "" {
		document.Version = canvasSpecVersion
	}

	spec, err := NormalizeCanvasSpec(document.Spec)
	if err != nil {
		return CanvasDocument{}, err
	}
	document.Spec = spec
	document.Selection = normalizeCanvasSelection(document.Selection, spec.Nodes, spec.Edges)
	return document, nil
}

func normalizeCanvasFields(fields []CanvasField, _ string) []CanvasField {
	normalized := make([]CanvasField, 0, len(fields))
	for _, field := range fields {
		field.Name = strings.TrimSpace(field.Name)
		field.Alias = strings.TrimSpace(field.Alias)
		if field.Name == "" {
			continue
		}
		normalized = append(normalized, CanvasField{
			Name:  field.Name,
			Alias: field.Alias,
		})
	}
	return normalized
}

func normalizeCanvasNode(node CanvasNode) CanvasNode {
	node.ID = strings.TrimSpace(node.ID)
	node.Kind = strings.TrimSpace(strings.ToLower(node.Kind))
	node.Table = strings.TrimSpace(node.Table)
	node.Alias = strings.TrimSpace(node.Alias)
	node.X = normalizeCanvasCoordinate(node.X)
	node.Y = normalizeCanvasCoordinate(node.Y)
	node.Width = normalizeCanvasDimension(node.Width, canvasNodeDefaultWidth)
	node.Height = normalizeCanvasDimension(node.Height, canvasNodeDefaultHeight)

	if node.Kind == "" {
		node.Kind = CanvasNodeKindTable
	}
	if node.Alias == "" {
		node.Alias = node.Table
	}
	node.Fields = normalizeCanvasFields(node.Fields, node.Alias)
	node.SelectedFields = normalizeSelectedFields(node.SelectedFields)
	return node
}

func normalizeCanvasDocumentTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

func normalizeCanvasCoordinate(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeCanvasDimension(value float64, fallback float64) float64 {
	if value <= 0 {
		return fallback
	}
	if value < canvasNodeMinSize {
		return canvasNodeMinSize
	}
	return value
}

func normalizeSelectedFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key := strings.ToLower(field)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, field)
	}
	return normalized
}

func filterSelectedFields(selectedFields []string, fields []CanvasField) []string {
	if len(selectedFields) == 0 || len(fields) == 0 {
		return nil
	}
	fieldSet := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		fieldSet[strings.ToLower(strings.TrimSpace(field.Name))] = struct{}{}
	}
	filtered := make([]string, 0, len(selectedFields))
	seen := map[string]struct{}{}
	for _, selected := range selectedFields {
		key := strings.ToLower(strings.TrimSpace(selected))
		if key == "" {
			continue
		}
		if _, exists := fieldSet[key]; !exists {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, strings.TrimSpace(selected))
	}
	return filtered
}

func normalizeCanvasSelection(selection CanvasSelection, nodes []CanvasNode, edges []CanvasEdge) CanvasSelection {
	nodeIDs := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		nodeIDs[node.ID] = struct{}{}
	}
	edgeIDs := make(map[string]struct{}, len(edges))
	for _, edge := range edges {
		edgeIDs[edge.ID] = struct{}{}
	}

	if _, exists := nodeIDs[selection.ActiveNodeID]; !exists {
		selection.ActiveNodeID = ""
	}
	if _, exists := edgeIDs[selection.ActiveEdgeID]; !exists {
		selection.ActiveEdgeID = ""
	}
	selection.SelectedNodeIDs = filterSelectionIDs(selection.SelectedNodeIDs, nodeIDs)
	selection.SelectedEdgeIDs = filterSelectionIDs(selection.SelectedEdgeIDs, edgeIDs)
	return selection
}

func filterSelectionIDs(values []string, valid map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, exists := valid[id]; !exists {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, id)
	}
	return filtered
}

func canvasNodeHasField(node CanvasNode, fieldName string) bool {
	if strings.TrimSpace(fieldName) == "" {
		return false
	}
	if len(node.Fields) == 0 {
		return true
	}
	for _, field := range node.Fields {
		if strings.EqualFold(field.Name, strings.TrimSpace(fieldName)) {
			return true
		}
	}
	return false
}

// ParseJoinType normalizes supported join types.
func ParseJoinType(raw string) (JoinType, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	switch raw {
	case "", "INNER", "INNER JOIN":
		return JoinInner, nil
	case "LEFT", "LEFT JOIN":
		return JoinLeft, nil
	case "RIGHT", "RIGHT JOIN":
		return JoinRight, nil
	case "FULL", "FULL JOIN", "FULL OUTER", "FULL OUTER JOIN":
		return JoinFull, nil
	default:
		return "", fmt.Errorf("unsupported join type: %s", raw)
	}
}

func normalizeCanvasEdge(edge CanvasEdge) CanvasEdge {
	edge.ID = strings.TrimSpace(edge.ID)
	edge.Kind = strings.TrimSpace(strings.ToLower(edge.Kind))
	edge.FromNode = strings.TrimSpace(edge.FromNode)
	edge.ToNode = strings.TrimSpace(edge.ToNode)
	edge.FromColumn = strings.TrimSpace(edge.FromColumn)
	edge.ToColumn = strings.TrimSpace(edge.ToColumn)
	edge.JoinType = strings.TrimSpace(strings.ToUpper(edge.JoinType))
	edge.Label = strings.TrimSpace(edge.Label)
	edge.Description = strings.TrimSpace(edge.Description)
	edge.Cardinality = strings.TrimSpace(edge.Cardinality)
	return edge
}
