package datasource

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/pocketbase/dbx"
)

func TestBuildFilter(t *testing.T) {
	scenarios := []struct {
		name     string
		filter   string
		dialect  string
		expect   []string // plain substrings present in rendered SQL
		hasWhere bool
		wantErr  bool
	}{
		{
			"simple equality",
			"name = 'iphone'",
			dialectPostgres,
			[]string{"name", "="},
			true,
			false,
		},
		{
			"null filter",
			"",
			dialectMySQL,
			nil,
			false,
			false,
		},
		{
			"comparison + and",
			"price > 100 && name ~ 'apple'",
			dialectMySQL,
			[]string{"AND"},
			true,
			false,
		},
		{
			"or grouping",
			"(name = 'a' || name = 'b') && price < 5",
			dialectPostgres,
			[]string{"OR", "AND"},
			true,
			false,
		},
		{
			"unsupported operator",
			"name ?= 'value'",
			dialectPostgres,
			nil,
			false,
			true,
		},
		{
			"invalid column chars",
			"na[me] = 'x'",
			dialectPostgres,
			nil,
			false,
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			expr, err := buildFilter(s.filter, s.dialect)
			if s.wantErr {
				if err == nil {
					t.Fatal("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if s.hasWhere {
				if expr == nil {
					t.Fatal("expected a non-nil expression")
				}
				sql := testSQL(expr)
				for _, e := range s.expect {
					if !strings.Contains(sql, e) {
						t.Errorf("expected rendered SQL to contain %q, got %s", e, sql)
					}
				}
				return
			}

			if expr != nil {
				t.Fatalf("expected nil expression for empty filter, got %v", expr)
			}
		})
	}
}

// testSQL renders a SelectQuery with the provided WHERE expression.
func testSQL(expr dbx.Expression) string {
	db := dbx.NewFromDB(&sql.DB{}, "sqlite3")
	return db.Select("*").From("external_table").Where(expr).Build().SQL()
}
