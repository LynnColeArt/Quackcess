package query

import (
	"fmt"
	"strings"
)

type PredicateOperator string

type JoinType string

const (
	JoinInner JoinType = "INNER JOIN"
	JoinLeft  JoinType = "LEFT JOIN"
	JoinRight JoinType = "RIGHT JOIN"
	JoinFull  JoinType = "FULL OUTER JOIN"

	PredicateEq       PredicateOperator = "="
	PredicateNeq      PredicateOperator = "!="
	PredicateGt       PredicateOperator = ">"
	PredicateGte      PredicateOperator = ">="
	PredicateLt       PredicateOperator = "<"
	PredicateLte      PredicateOperator = "<="
	PredicateLike     PredicateOperator = "LIKE"
	PredicateIn       PredicateOperator = "IN"
	PredicateContains PredicateOperator = "CONTAINS"
	PredicateIsNull   PredicateOperator = "IS NULL"
	PredicateNotNull  PredicateOperator = "IS NOT NULL"
)

type Predicate struct {
	Expression string
	Field      FieldRef
	Operator   PredicateOperator
	Values     []any
	Not        bool
}

type QuerySource struct {
	Table string
	Alias string
}

type QueryGraph struct {
	From       QuerySource
	Joins      []Join
	Fields     []FieldRef
	Where      string
	Predicates []Predicate
	GroupBy    []FieldRef
	OrderBy    []OrderBy
	Limit      int
}

type Join struct {
	Type        JoinType
	LeftAlias   string
	LeftColumn  string
	RightTable  string
	RightAlias  string
	RightColumn string
}

type FieldRef struct {
	Expression string
	Alias      string
	Source     string
	Column     string
}

type OrderBy struct {
	Expression string
	Source     string
	Column     string
	Desc       bool
}

func ValidateGraph(graph QueryGraph) error {
	if _, err := NormalizeGraph(graph); err != nil {
		return err
	}
	return nil
}

func NormalizeGraph(graph QueryGraph) (QueryGraph, error) {
	if strings.TrimSpace(graph.From.Table) == "" {
		return QueryGraph{}, fmt.Errorf("from table is required")
	}

	sourceAlias := strings.TrimSpace(graph.From.Alias)
	if sourceAlias == "" {
		sourceAlias = graph.From.Table
	}

	normalized := graph
	normalized.From.Alias = sourceAlias
	normalized.Joins = nil

	seenAliases := map[string]struct{}{
		sourceAlias: {},
	}

	for _, join := range graph.Joins {
		normalizedJoin, err := normalizeJoin(join)
		if err != nil {
			return QueryGraph{}, err
		}
		normalizedJoin.LeftAlias = strings.TrimSpace(normalizedJoin.LeftAlias)
		if normalizedJoin.LeftAlias == "" {
			normalizedJoin.LeftAlias = sourceAlias
		}
		if _, exists := seenAliases[normalizedJoin.LeftAlias]; !exists {
			return QueryGraph{}, fmt.Errorf("unknown join left source alias: %s", normalizedJoin.LeftAlias)
		}
		if _, exists := seenAliases[normalizedJoin.RightAlias]; exists {
			return QueryGraph{}, fmt.Errorf("join alias already declared: %s", normalizedJoin.RightAlias)
		}
		seenAliases[normalizedJoin.RightAlias] = struct{}{}
		normalized.Joins = append(normalized.Joins, normalizedJoin)
	}

	allRefs := append([]FieldRef{}, graph.Fields...)
	allRefs = append(allRefs, graph.GroupBy...)
	for _, orderBy := range graph.OrderBy {
		allRefs = append(allRefs, fieldRefFromOrderBy(orderBy))
	}
	for _, predicate := range graph.Predicates {
		if strings.TrimSpace(predicate.Expression) != "" || strings.TrimSpace(predicate.Field.Expression) != "" {
			continue
		}
		if err := validatePredicateField(predicate.Field); err != nil {
			return QueryGraph{}, err
		}
		if _, exists := seenAliases[predicate.Field.Source]; !exists {
			return QueryGraph{}, fmt.Errorf("unknown source alias: %s", predicate.Field.Source)
		}
		if err := validatePredicateValues(predicate); err != nil {
			return QueryGraph{}, err
		}
	}
	for _, field := range allRefs {
		if strings.TrimSpace(field.Expression) != "" {
			continue
		}
		if field.Column == "*" {
			continue
		}
		if field.Column == "" {
			return QueryGraph{}, fmt.Errorf("field column is required")
		}
		if field.Source == "" {
			return QueryGraph{}, fmt.Errorf("field source is required")
		}
		if _, exists := seenAliases[field.Source]; !exists {
			return QueryGraph{}, fmt.Errorf("unknown source alias: %s", field.Source)
		}
	}

	if strings.TrimSpace(graph.Where) != "" && len(graph.Predicates) > 0 {
		return QueryGraph{}, fmt.Errorf("either where string or predicates may be set, not both")
	}

	return normalized, nil
}

func normalizeJoin(join Join) (Join, error) {
	if strings.TrimSpace(string(join.Type)) == "" {
		join.Type = JoinInner
	} else {
		switch strings.TrimSpace(string(join.Type)) {
		case string(JoinInner), string(JoinLeft), string(JoinRight), string(JoinFull):
		default:
			return Join{}, fmt.Errorf("unsupported join type: %s", join.Type)
		}
	}

	if strings.TrimSpace(join.LeftAlias) == "" {
		return Join{}, fmt.Errorf("join left source alias is required")
	}
	if strings.TrimSpace(join.LeftColumn) == "" {
		return Join{}, fmt.Errorf("join left column is required")
	}
	if strings.TrimSpace(join.RightTable) == "" {
		return Join{}, fmt.Errorf("join right table is required")
	}
	if strings.TrimSpace(join.RightAlias) == "" {
		return Join{}, fmt.Errorf("join right source alias is required")
	}
	if strings.TrimSpace(join.RightColumn) == "" {
		return Join{}, fmt.Errorf("join right column is required")
	}
	return join, nil
}

func validatePredicateField(field FieldRef) error {
	if field.Column == "" {
		return fmt.Errorf("predicate column is required")
	}
	if field.Source == "" {
		return fmt.Errorf("predicate source is required")
	}
	return nil
}

func validatePredicateValues(predicate Predicate) error {
	if err := validatePredicateOperator(predicate.Operator); err != nil {
		return err
	}

	switch predicate.Operator {
	case PredicateIsNull, PredicateNotNull:
		if len(predicate.Values) > 0 {
			return fmt.Errorf("%s predicate does not accept values", predicate.Operator)
		}
	case PredicateIn:
		if len(predicate.Values) == 0 {
			return fmt.Errorf("IN predicate requires at least one value")
		}
	default:
		if len(predicate.Values) == 0 {
			return fmt.Errorf("predicate requires at least one value")
		}
		if len(predicate.Values) > 1 {
			return fmt.Errorf("predicate requires exactly one value")
		}
	}
	return nil
}

func validatePredicateOperator(op PredicateOperator) error {
	switch op {
	case PredicateEq, PredicateNeq, PredicateGt, PredicateGte, PredicateLt, PredicateLte, PredicateLike, PredicateIn, PredicateContains, PredicateIsNull, PredicateNotNull:
		return nil
	default:
		return fmt.Errorf("unsupported predicate operator: %s", op)
	}
}

func fieldRefFromOrderBy(orderBy OrderBy) FieldRef {
	if orderBy.Expression != "" {
		return FieldRef{Expression: orderBy.Expression}
	}
	return FieldRef{Source: orderBy.Source, Column: orderBy.Column}
}
