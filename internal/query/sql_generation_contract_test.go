package query

import "testing"

func TestGenerateSQLFromGraph(t *testing.T) {
	graph := QueryGraph{
		From: QuerySource{Table: "customers", Alias: "c"},
		Joins: []Join{
			{
				Type:        JoinLeft,
				LeftAlias:   "c",
				LeftColumn:  "id",
				RightTable:  "orders",
				RightAlias:  "o",
				RightColumn: "customer_id",
			},
		},
		Fields: []FieldRef{
			{Source: "c", Column: "id"},
			{Source: "c", Column: "name", Alias: "customer_name"},
			{Expression: "COUNT(*)", Alias: "order_count"},
		},
		Where: "o.total > 0",
		GroupBy: []FieldRef{
			{Source: "c", Column: "id"},
			{Source: "c", Column: "name"},
		},
		OrderBy: []OrderBy{
			{Source: "c", Column: "name", Desc: true},
		},
		Limit: 25,
	}

	got, err := GenerateSQL(graph)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	want := `SELECT "c"."id", "c"."name" AS "customer_name", (COUNT(*)) AS "order_count" FROM "customers" "c" LEFT JOIN "orders" "o" ON "c"."id" = "o"."customer_id" WHERE o.total > 0 GROUP BY "c"."id", "c"."name" ORDER BY "c"."name" DESC LIMIT 25`
	if got.SQL != want {
		t.Fatalf("sql mismatch.\nwant: %s\ngot:  %s", want, got.SQL)
	}
	if len(got.Parameters) != 0 {
		t.Fatalf("unexpected parameters: %#v", got.Parameters)
	}
}

func TestGenerateSQLFromPredicates(t *testing.T) {
	graph := QueryGraph{
		From: QuerySource{Table: "orders", Alias: "o"},
		Fields: []FieldRef{
			{Source: "o", Column: "id"},
		},
		Predicates: []Predicate{
			{
				Field:    FieldRef{Source: "o", Column: "status"},
				Operator: PredicateIn,
				Values:   []any{"pending", "shipped"},
			},
			{
				Field:    FieldRef{Source: "o", Column: "sku"},
				Operator: PredicateContains,
				Values:   []any{"ABC"},
			},
			{
				Field:    FieldRef{Source: "o", Column: "is_active"},
				Operator: PredicateEq,
				Not:      true,
				Values:   []any{true},
			},
		},
		OrderBy: []OrderBy{
			{Source: "o", Column: "id"},
		},
	}

	got, err := GenerateSQL(graph)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	wantSQL := `SELECT "o"."id" FROM "orders" "o" WHERE "o"."status" IN (?, ?) AND "o"."sku" LIKE ? AND "o"."is_active" != ? ORDER BY "o"."id" ASC`
	if got.SQL != wantSQL {
		t.Fatalf("sql mismatch.\nwant: %s\ngot:  %s", wantSQL, got.SQL)
	}
	wantParams := []any{"pending", "shipped", "%ABC%", true}
	if gotlen := len(got.Parameters); gotlen != len(wantParams) {
		t.Fatalf("parameter count = %d, want %d", gotlen, len(wantParams))
	}
	for i, gotValue := range got.Parameters {
		if gotValue != wantParams[i] {
			t.Fatalf("parameter[%d] = %#v, want %#v", i, gotValue, wantParams[i])
		}
	}
}
