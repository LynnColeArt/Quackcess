package query

import (
	"fmt"
	"strings"
)

type QuerySQL struct {
	SQL        string
	Parameters []any
}

func GenerateSQL(graph QueryGraph) (QuerySQL, error) {
	normalized, err := NormalizeGraph(graph)
	if err != nil {
		return QuerySQL{}, err
	}

	var builder strings.Builder
	builder.WriteString("SELECT ")
	if len(normalized.Fields) == 0 {
		builder.WriteString("*")
	} else {
		builder.WriteString(renderFieldList(normalized.Fields))
	}

	builder.WriteString(" FROM ")
	builder.WriteString(quoteIdentifier(normalized.From.Table))
	builder.WriteString(" ")
	builder.WriteString(quoteIdentifier(normalized.From.Alias))

	for _, join := range normalized.Joins {
		builder.WriteString(fmt.Sprintf(" %s ", join.Type))
		builder.WriteString(quoteIdentifier(join.RightTable))
		builder.WriteString(" ")
		builder.WriteString(quoteIdentifier(join.RightAlias))
		builder.WriteString(" ON ")
		builder.WriteString(joinOnClause(normalized.From.Table, join, normalized.From.Alias, quoteJoinAlias))
	}

	parameters := make([]any, 0)
	if strings.TrimSpace(normalized.Where) != "" {
		builder.WriteString(" WHERE ")
		builder.WriteString(strings.TrimSpace(normalized.Where))
	} else if len(normalized.Predicates) > 0 {
		whereClause, err := renderPredicates(normalized.Predicates)
		if err != nil {
			return QuerySQL{}, err
		}
		builder.WriteString(" WHERE ")
		builder.WriteString(whereClause.clause)
		parameters = append(parameters, whereClause.args...)
	}

	if len(normalized.GroupBy) > 0 {
		builder.WriteString(" GROUP BY ")
		builder.WriteString(renderFieldList(normalized.GroupBy))
	}

	if len(normalized.OrderBy) > 0 {
		builder.WriteString(" ORDER BY ")
		builder.WriteString(renderOrderBy(normalized.OrderBy))
	}

	if normalized.Limit > 0 {
		builder.WriteString(fmt.Sprintf(" LIMIT %d", normalized.Limit))
	}

	return QuerySQL{SQL: builder.String(), Parameters: parameters}, nil
}

type sqlClause struct {
	clause string
	args   []any
}

func renderPredicates(predicates []Predicate) (sqlClause, error) {
	parts := make([]string, 0, len(predicates))
	args := make([]any, 0, len(predicates))

	for _, predicate := range predicates {
		if strings.TrimSpace(predicate.Expression) != "" {
			parts = append(parts, predicate.Expression)
			continue
		}

		field, err := renderPredicatedField(predicate.Field)
		if err != nil {
			return sqlClause{}, err
		}
		operator := predicate.Operator

		operator, notPrefix, err := applyNotModifier(operator, predicate.Not)
		if err != nil {
			return sqlClause{}, err
		}

		switch operator {
		case PredicateIsNull, PredicateNotNull:
			parts = append(parts, fmt.Sprintf("%s %s", field, operator))
		case PredicateIn:
			if len(predicate.Values) == 0 {
				return sqlClause{}, fmt.Errorf("IN predicate requires at least one value")
			}
			placeholders := make([]string, 0, len(predicate.Values))
			for _, value := range predicate.Values {
				args = append(args, value)
				placeholders = append(placeholders, "?")
			}
			parts = append(parts, fmt.Sprintf("%s %sIN (%s)", field, notPrefix, strings.Join(placeholders, ", ")))
		case PredicateContains, PredicateLike:
			if len(predicate.Values) != 1 {
				return sqlClause{}, fmt.Errorf("predicate value count mismatch")
			}
			args = append(args, wildcardString(predicate.Values[0]))
			parts = append(parts, fmt.Sprintf("%s %sLIKE ?", field, notPrefix))
		default:
			if len(predicate.Values) != 1 {
				return sqlClause{}, fmt.Errorf("predicate requires exactly one value")
			}
			parts = append(parts, fmt.Sprintf("%s %s ?", field, operator))
			args = append(args, predicate.Values[0])
		}
	}

	return sqlClause{clause: strings.Join(parts, " AND "), args: args}, nil
}

func applyNotModifier(op PredicateOperator, negate bool) (predicate PredicateOperator, notPrefix string, err error) {
	predicate = op
	notPrefix = ""
	if !negate {
		return predicate, notPrefix, nil
	}

	switch op {
	case PredicateEq:
		return PredicateNeq, notPrefix, nil
	case PredicateNeq:
		return PredicateEq, notPrefix, nil
	case PredicateGt:
		return PredicateLte, notPrefix, nil
	case PredicateGte:
		return PredicateLt, notPrefix, nil
	case PredicateLt:
		return PredicateGte, notPrefix, nil
	case PredicateLte:
		return PredicateGt, notPrefix, nil
	case PredicateIn:
		return PredicateIn, "NOT ", nil
	case PredicateContains:
		return PredicateLike, "NOT ", nil
	case PredicateLike:
		return PredicateLike, "NOT ", nil
	case PredicateIsNull:
		return PredicateNotNull, "", nil
	case PredicateNotNull:
		return PredicateIsNull, "", nil
	default:
		return "", "", fmt.Errorf("unsupported predicate operator: %s", op)
	}
}

func wildcardString(value any) string {
	return "%" + fmt.Sprintf("%v", value) + "%"
}

func renderPredicatedField(field FieldRef) (string, error) {
	if strings.TrimSpace(field.Expression) != "" {
		return "(" + field.Expression + ")", nil
	}
	if field.Source == "" || field.Column == "" {
		return "", fmt.Errorf("predicate field requires source and column")
	}
	if field.Column == "*" {
		return quoteIdentifier(field.Source), nil
	}
	return fmt.Sprintf("%s.%s", quoteIdentifier(field.Source), quoteIdentifier(field.Column)), nil
}

func renderFieldList(fields []FieldRef) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, renderField(field))
	}
	return strings.Join(parts, ", ")
}

func renderField(field FieldRef) string {
	if strings.TrimSpace(field.Expression) != "" {
		if field.Alias != "" {
			return fmt.Sprintf("(%s) AS %s", field.Expression, quoteIdentifier(field.Alias))
		}
		return fmt.Sprintf("(%s)", field.Expression)
	}
	if field.Column == "*" {
		if field.Alias != "" {
			return fmt.Sprintf("* AS %s", quoteIdentifier(field.Alias))
		}
		return "*"
	}
	return fmt.Sprintf("%s.%s", quoteIdentifier(field.Source), quoteIdentifier(field.Column)) + aliasSuffix(field.Alias)
}

func renderFieldNoAlias(field FieldRef) string {
	if strings.TrimSpace(field.Expression) != "" {
		return fmt.Sprintf("(%s)", field.Expression)
	}
	if field.Column == "*" {
		return "*"
	}
	return fmt.Sprintf("%s.%s", quoteIdentifier(field.Source), quoteIdentifier(field.Column))
}

func aliasSuffix(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return ""
	}
	return fmt.Sprintf(" AS %s", quoteIdentifier(alias))
}

func renderOrderBy(orderBy []OrderBy) string {
	parts := make([]string, 0, len(orderBy))
	for _, item := range orderBy {
		expr := renderFieldNoAlias(fieldRefFromOrderBy(item))
		if item.Desc {
			expr += " DESC"
		} else {
			expr += " ASC"
		}
		parts = append(parts, expr)
	}
	return strings.Join(parts, ", ")
}

func quoteJoinAlias(alias string) string {
	return quoteIdentifier(alias)
}

func joinOnClause(fromTable string, join Join, fromAlias string, quoteAlias func(string) string) string {
	_ = fromTable
	leftAlias := strings.TrimSpace(join.LeftAlias)
	rightAlias := strings.TrimSpace(join.RightAlias)
	if leftAlias == "" {
		leftAlias = fromAlias
	}
	return fmt.Sprintf("%s.%s = %s.%s",
		quoteAlias(leftAlias),
		quoteIdentifier(join.LeftColumn),
		quoteAlias(rightAlias),
		quoteIdentifier(join.RightColumn),
	)
}

func quoteIdentifier(value string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(value, `"`, `""`))
}
