package datasource

import (
	"fmt"
	"strings"

	"github.com/ganigeorgiev/fexpr"
	"github.com/pocketbase/dbx"
	"github.com/spf13/cast"
)

// dialect detection (the driver name doubles as the SQL dialect name).
const (
	dialectPostgres = "pgx"
	dialectMySQL    = "mysql"
	dialectMSSQL    = "sqlserver"
)

// buildFilter parses the provided filter string (fexpr syntax) over plain
// external table columns and returns a dialect-aware dbx WHERE Expression.
//
// Supported operators:
//   - =, !=, >, >=, <, <=
//   - ~ (contains, case-insensitive), !~ (not contains)
//
// "&&" / "||" combinators and bracketed grouping are supported. Array "any"
// operators, functions and multi-match expressions are not supported yet.
func buildFilter(filter string, dialect string) (dbx.Expression, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil, nil
	}

	b := &filterBuilder{
		params: dbx.Params{},
	}
	if dialect == dialectPostgres {
		b.likeOp = "ILIKE"
	} else {
		b.likeOp = "LIKE"
	}
	if dialect == dialectMySQL {
		b.concat = "CONCAT('%', %s, '%')"
	} else {
		b.concat = "'%' || %s || '%'"
	}

	groups, err := fexpr.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to parse filter: %w", err)
	}

	expr, err := b.groups(groups)
	if err != nil {
		return nil, err
	}

	return expr, nil
}

type filterBuilder struct {
	params  dbx.Params
	counter int
	likeOp  string
	concat  string
}

// groups builds an AND/OR combined expression from a group list.
func (b *filterBuilder) groups(groups []fexpr.ExprGroup) (dbx.Expression, error) {
	if len(groups) == 0 {
		return nil, fmt.Errorf("empty filter expression")
	}

	var result dbx.Expression

	for i, group := range groups {
		var expr dbx.Expression
		var exprErr error

		switch item := group.Item.(type) {
		case fexpr.Expr:
			expr, exprErr = b.expr(item)
		case []fexpr.ExprGroup:
			expr, exprErr = b.groups(item)
		default:
			exprErr = fmt.Errorf("unsupported filter expression item")
		}

		if exprErr != nil {
			return nil, exprErr
		}

		if i == 0 {
			result = expr
		} else if group.Join == fexpr.JoinOr {
			result = dbx.Or(result, expr)
		} else {
			result = dbx.And(result, expr)
		}
	}

	return result, nil
}

func (b *filterBuilder) expr(e fexpr.Expr) (dbx.Expression, error) {
	left, lErr := b.operand(e.Left)
	if lErr != nil {
		return nil, lErr
	}
	right, rErr := b.operand(e.Right)
	if rErr != nil {
		return nil, rErr
	}

	switch e.Op {
	case fexpr.SignEq:
		return dbx.NewExp(fmt.Sprintf("%s = %s", left, right), b.params), nil
	case fexpr.SignNeq:
		return dbx.NewExp(fmt.Sprintf("%s IS NOT %s", left, right), b.params), nil
	case fexpr.SignLt:
		return dbx.NewExp(fmt.Sprintf("%s < %s", left, right), b.params), nil
	case fexpr.SignLte:
		return dbx.NewExp(fmt.Sprintf("%s <= %s", left, right), b.params), nil
	case fexpr.SignGt:
		return dbx.NewExp(fmt.Sprintf("%s > %s", left, right), b.params), nil
	case fexpr.SignGte:
		return dbx.NewExp(fmt.Sprintf("%s >= %s", left, right), b.params), nil
	case fexpr.SignLike:
		return b.like(left, e.Right), nil
	case fexpr.SignNlike:
		return b.notLike(left, e.Right), nil
	}

	return nil, fmt.Errorf("unsupported filter operator %q", e.Op)
}

// like builds "left LIKE pattern" where:
//   - literal right value is wrapped with % for a contains match
//   - column right operand uses a dialect-aware concat
func (b *filterBuilder) like(leftSQL string, right fexpr.Token) dbx.Expression {
	return b.likeWhere(leftSQL, right, false)
}

func (b *filterBuilder) notLike(leftSQL string, right fexpr.Token) dbx.Expression {
	return b.likeWhere(leftSQL, right, true)
}

func (b *filterBuilder) likeWhere(leftSQL string, right fexpr.Token, negate bool) dbx.Expression {
	op := b.likeOp
	if negate {
		op = "NOT " + b.likeOp
	}

	// literal value -> bind the full %pattern% as a single param
	if value, ok := b.literalValue(right); ok {
		ph := b.param("%" + value + "%")
		return dbx.NewExp(fmt.Sprintf("%s %s %s", leftSQL, op, ph), b.params)
	}

	// column right operand -> concat the surrounding % via the dialect builder
	rightSQL, _ := b.operand(right)
	return dbx.NewExp(
		fmt.Sprintf("%s %s %s", leftSQL, op, fmt.Sprintf(b.concat, rightSQL)),
		b.params,
	)
}

// operand resolves a token into a quoted SQL column, a bound-value
// placeholder, or a NULL/TRUE/FALSE literal.
func (b *filterBuilder) operand(t fexpr.Token) (string, error) {
	switch t.Type {
	case fexpr.TokenIdentifier:
		switch strings.ToLower(t.Literal) {
		case "null":
			return "NULL", nil
		case "true":
			return "1", nil
		case "false":
			return "0", nil
		}
		col := sanitizeColumn(t.Literal)
		if col == "" {
			return "", fmt.Errorf("invalid column name %q", t.Literal)
		}
		return "[[" + col + "]]", nil
	case fexpr.TokenText:
		return b.param(t.Literal), nil
	case fexpr.TokenNumber:
		return b.param(cast.ToFloat64(t.Literal)), nil
	}

	return "", fmt.Errorf("unsupported filter operand %q", t.Literal)
}

// sanitizeColumn validates and normalizes a column identifier to only
// allow safe characters (letters, digits, underscore, dash, dot).
func sanitizeColumn(name string) string {
	name = strings.Trim(name, "`\"[]")
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return ""
		}
	}
	return name
}

// literalValue returns the go value for text/number tokens (used for the
// %pattern% wrapping of the contains operators).
func (b *filterBuilder) literalValue(t fexpr.Token) (string, bool) {
	switch t.Type {
	case fexpr.TokenText:
		return t.Literal, true
	case fexpr.TokenNumber:
		return cast.ToString(t.Literal), true
	}
	return "", false
}

// param binds a value and returns its {:name} placeholder.
func (b *filterBuilder) param(value any) string {
	b.counter++
	name := fmt.Sprintf("nbx_f%d", b.counter)
	b.params[name] = value
	return "{:" + name + "}"
}
