package query

import (
	"fmt"
	"strings"
)

// QueryGraphFromCanvas converts a canvas spec into a query graph.
func QueryGraphFromCanvas(raw CanvasSpec) (QueryGraph, error) {
	spec, err := NormalizeCanvasSpec(raw)
	if err != nil {
		return QueryGraph{}, err
	}

	nodeByID := make(map[string]CanvasNode, len(spec.Nodes))
	for _, node := range spec.Nodes {
		nodeByID[node.ID] = node
	}

	rootNode := spec.Nodes[0]
	graph := QueryGraph{
		From: QuerySource{
			Table: rootNode.Table,
			Alias: rootNode.Alias,
		},
		Fields: fieldsFromCanvasNodes(spec.Nodes),
	}
	graph.Fields = dedupeFieldRefs(graph.Fields)

	if len(graph.Fields) == 0 {
		graph.Fields = append(graph.Fields, FieldRef{Source: rootNode.Alias, Column: "*"})
	}

	for _, edge := range spec.Edges {
		if edge.Kind != CanvasEdgeKindJoin {
			return QueryGraph{}, fmt.Errorf("unsupported edge kind: %s", edge.Kind)
		}
		leftNode, rightNode := nodeByID[edge.FromNode], nodeByID[edge.ToNode]
		if leftNode.ID == "" || rightNode.ID == "" {
			return QueryGraph{}, fmt.Errorf("edge references missing nodes: %s -> %s", edge.FromNode, edge.ToNode)
		}
		joinType, err := ParseJoinType(edge.JoinType)
		if err != nil {
			return QueryGraph{}, err
		}
		graph.Joins = append(graph.Joins, Join{
			Type:        joinType,
			LeftAlias:   leftNode.Alias,
			LeftColumn:  edge.FromColumn,
			RightTable:  rightNode.Table,
			RightAlias:  rightNode.Alias,
			RightColumn: edge.ToColumn,
		})
	}

	return graph, nil
}

// GenerateSQLFromCanvas renders SQL from a canvas specification.
func GenerateSQLFromCanvas(spec CanvasSpec) (QuerySQL, error) {
	graph, err := QueryGraphFromCanvas(spec)
	if err != nil {
		return QuerySQL{}, err
	}
	return GenerateSQL(graph)
}

// GenerateSQLFromCanvasWithLimit renders SQL from a canvas specification and applies a row limit.
// It keeps the generated SQL predictable while capping result sets during live previews.
func GenerateSQLFromCanvasWithLimit(spec CanvasSpec, limit int) (QuerySQL, error) {
	graph, err := QueryGraphFromCanvas(spec)
	if err != nil {
		return QuerySQL{}, err
	}
	if limit > 0 {
		graph.Limit = limit
	}
	return GenerateSQL(graph)
}

func fieldsFromCanvasNodes(nodes []CanvasNode) []FieldRef {
	out := make([]FieldRef, 0)
	for _, node := range nodes {
		nodeFields := node.Fields
		if len(node.SelectedFields) > 0 {
			nodeFields = selectFieldsForCanvasNode(node)
		}
		for _, field := range nodeFields {
			out = append(out, FieldRef{
				Source: node.Alias,
				Column: field.Name,
				Alias:  field.Alias,
			})
		}
	}
	return out
}

func selectFieldsForCanvasNode(node CanvasNode) []CanvasField {
	fieldSet := make(map[string]CanvasField, len(node.Fields))
	for _, field := range node.Fields {
		fieldSet[strings.ToLower(strings.TrimSpace(field.Name))] = field
	}

	out := make([]CanvasField, 0, len(node.SelectedFields))
	for _, selected := range node.SelectedFields {
		field, ok := fieldSet[strings.ToLower(strings.TrimSpace(selected))]
		if !ok || field.Name == "" {
			continue
		}
		out = append(out, field)
	}
	return out
}

func dedupeFieldRefs(fields []FieldRef) []FieldRef {
	if len(fields) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]FieldRef, 0, len(fields))
	for _, field := range fields {
		key := field.Source + "." + field.Column + "@" + field.Alias
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, field)
	}
	return out
}
