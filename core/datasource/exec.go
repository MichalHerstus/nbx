package datasource

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/search"
)

// List executes a read-only, paginated list (browse/search/sort) query
// against an external SQL datasource and hydrates the rows into records.
func (r *Registry) List(
	collection *core.Collection,
	cred core.Credential,
	page int,
	perPage int,
	sort string,
	filter string,
) (*search.Result, error) {
	ds := collection.GetDataSource()

	if ds.IsREST() {
		return r.listREST(collection, ds, page, perPage, sort, filter)
	}

	db, err := r.Get(ds, cred)
	if err != nil {
		return nil, err
	}

	table := strings.TrimSpace(ds.Table)
	if table == "" {
		return nil, fmt.Errorf("missing datasource table definition")
	}
	table = sanitizeTable(table)

	dialect := driverFor(ds.Type)

	where, err := buildFilter(filter, dialect)
	if err != nil {
		return nil, err
	}

	// count
	var totals []int64
	countQuery := db.Select("COUNT(*)").From(table)
	if where != nil {
		countQuery.Where(where)
	}
	if err := countQuery.Column(&totals); err != nil {
		return nil, fmt.Errorf("failed to count external records: %w", err)
	}
	var total int
	if len(totals) > 0 {
		total = int(totals[0])
	}

	// rows
	query := db.Select("*").From(table)
	if where != nil {
		query.Where(where)
	}

	orderBy := parseSort(sort, collection)
	if len(orderBy) > 0 {
		query.OrderBy(orderBy...)
	} else if dialect == dialectMSSQL && perPage > 0 {
		// mssql requires ORDER BY when using OFFSET/FETCH
		query.OrderBy(defaultOrderBy(collection))
	}

	if perPage > 0 {
		query.Limit(int64(perPage)).Offset(int64((page - 1) * perPage))
	}

	rows := []dbx.NullStringMap{}
	if err := query.All(&rows); err != nil {
		return nil, fmt.Errorf("failed to query external records: %w", err)
	}

	records, err := core.RecordsFromNullStringMaps(collection, rows)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if perPage > 0 {
		totalPages = total / perPage
		if total%perPage != 0 {
			totalPages++
		}
	}

	return &search.Result{
		Items:      records,
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

// Find returns a single record by a value in the primary field (usually "id").
func (r *Registry) Find(collection *core.Collection, cred core.Credential, id string) (*core.Record, error) {
	ds := collection.GetDataSource()
	db, err := r.Get(ds, cred)
	if err != nil {
		return nil, err
	}

	table := sanitizeTable(strings.TrimSpace(ds.Table))
	if table == "" {
		return nil, fmt.Errorf("missing datasource table definition")
	}

	primary := primaryKeyName(collection)

	rows := []dbx.NullStringMap{}
	query := db.Select("*").From(table).
		Where(dbx.NewExp("[["+sanitizeColumn(primary)+"]] = {:nbx_id}", dbx.Params{"nbx_id": id})).
		Limit(1)
	if err := query.All(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	records, err := core.RecordsFromNullStringMaps(collection, rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// primaryKeyName returns the name of the collection primary key field
// (falling back to "id" when not explicitly marked).
func primaryKeyName(c *core.Collection) string {
	if c == nil {
		return core.FieldNameId
	}
	for _, f := range c.Fields {
		if tf, ok := f.(*core.TextField); ok && tf.PrimaryKey {
			return tf.Name
		}
	}
	return core.FieldNameId
}

// parseSort parses the `sort` query param into order by columns.
func parseSort(sort string, collection *core.Collection) []string {
	fields := []string{}

	for _, part := range strings.Split(sort, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		dir := "ASC"
		name := part
		if strings.HasPrefix(part, "-") {
			dir = "DESC"
			name = strings.TrimPrefix(part, "-")
		} else if strings.HasPrefix(part, "+") {
			name = strings.TrimPrefix(part, "+")
		}

		name = sanitizeColumn(name)
		if name == "" {
			continue
		}

		fields = append(fields, `[[`+name+`]] `+dir)
	}

	return fields
}

// defaultOrderBy returns a fallback ORDER BY used when paginating mssql
// without an explicit sort.
func defaultOrderBy(collection *core.Collection) string {
	return "[[" + sanitizeColumn(primaryKeyName(collection)) + "]] ASC"
}

// sanitizeTable validates and normalizes a table identifier to only allow
// safe characters.
func sanitizeTable(name string) string {
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return ""
		}
	}
	return name
}
