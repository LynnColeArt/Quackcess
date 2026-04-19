package query

import "testing"

func TestQueryGraphRequiresFromSource(t *testing.T) {
	if err := ValidateGraph(QueryGraph{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestQueryGraphDefaultsAliasAndValidatesJoins(t *testing.T) {
	graph := QueryGraph{
		From: QuerySource{Table: "customers"},
		Joins: []Join{
			{
				Type:        JoinLeft,
				LeftAlias:   "customers",
				LeftColumn:  "id",
				RightTable:  "orders",
				RightAlias:  "orders",
				RightColumn: "customer_id",
			},
		},
	}

	normalized, err := NormalizeGraph(graph)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.From.Alias != "customers" {
		t.Fatalf("alias = %q, want customers", normalized.From.Alias)
	}
}

func TestQueryGraphRejectsUnknownJoinAlias(t *testing.T) {
	graph := QueryGraph{
		From: QuerySource{Table: "customers", Alias: "c"},
		Joins: []Join{
			{
				Type:        JoinLeft,
				LeftAlias:   "unknown",
				LeftColumn:  "id",
				RightTable:  "orders",
				RightAlias:  "o",
				RightColumn: "customer_id",
			},
		},
	}
	if err := ValidateGraph(graph); err == nil {
		t.Fatal("expected unknown alias error")
	}
}

func TestQueryGraphRejectsWhereAndPredicatesTogether(t *testing.T) {
	graph := QueryGraph{
		From:  QuerySource{Table: "customers", Alias: "c"},
		Where: "1 = 1",
		Predicates: []Predicate{
			{
				Field:    FieldRef{Source: "c", Column: "id"},
				Operator: PredicateEq,
				Values:   []any{1},
			},
		},
	}
	if err := ValidateGraph(graph); err == nil {
		t.Fatal("expected both where/predicates rejection error")
	}
}
