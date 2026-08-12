package nbx

import (
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/core/datasource"
	"github.com/pocketbase/pocketbase/tools/search"
)

// maxAggregateRows caps how many external rows are fetched for aggregation.
const maxAggregateRows = 5000

// datasourceRegistry is the shared external datasource connection registry for
// the report/aggregation routes (mirrors apis/record_datasource.go).
var datasourceRegistry = datasource.NewRegistry()

// datasourceCredential resolves the named credential vault entry referenced by
// the datasource.
func datasourceCredential(app core.App, ds core.DataSource) core.Credential {
	if ds.CredentialRef == "" {
		return core.Credential{}
	}
	return app.Settings().Nbx.Secrets[ds.CredentialRef]
}

// loadWidgetRecords returns the records of a widget source collection
// honouring the widget filter/sort/perPage. Local collections use the record
// query + search provider; external ones go through the datasource registry.
func loadWidgetRecords(
	e *core.RequestEvent,
	app core.App,
	collection *core.Collection,
	widget core.NbxWidget,
	requestInfo *core.RequestInfo,
	perPage int,
) ([]*core.Record, error) {
	if !collection.GetDataSource().IsLocal() {
		cred := datasourceCredential(app, collection.GetDataSource())
		result, err := datasourceRegistry.List(collection, cred, 1, perPage, widget.Sort, widget.Filter)
		if err != nil {
			return nil, err
		}
		records, _ := result.Items.([]*core.Record)
		return records, nil
	}

	query := app.RecordQuery(collection)
	resolver := core.NewRecordFieldResolver(app, collection, requestInfo, true)
	resolver.SetAllowHiddenFields(true)

	if strings.TrimSpace(widget.Filter) != "" {
		expr, err := search.FilterData(widget.Filter).BuildExpr(resolver)
		if err != nil {
			return nil, err
		}
		query.AndWhere(expr)
		if err := resolver.UpdateQuery(query); err != nil {
			return nil, err
		}
	}

	if strings.TrimSpace(widget.Sort) != "" {
		sortCols := widgetSortToColumns(collection, widget.Sort)
		query.OrderBy(sortCols...)
	}

	if perPage > 0 {
		query.Limit(int64(perPage))
	}

	records := []*core.Record{}
	if err := query.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}

// widgetSortToColumns converts a widget sort spec to order by columns.
func widgetSortToColumns(collection *core.Collection, sort string) []string {
	result := []string{}
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
		result = append(result, "[["+name+"]] "+dir)
	}
	return result
}

// structFieldNames returns the displayable collection field names (excluding
// the "id" primary key column).
func structFieldNames(collection *core.Collection) []string {
	if collection == nil {
		return []string{}
	}
	names := []string{}
	for _, f := range collection.Fields {
		if f.GetName() == core.FieldNameId {
			continue
		}
		if f.GetName() == "updated" || f.GetName() == "created" {
			names = append(names, f.GetName())
			continue
		}
		names = append(names, f.GetName())
	}
	return names
}

// formatWidgetValue stringifies a record field value for table/pdf output.
func formatWidgetValue(value any, field string) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case map[string]any:
		return fmt.Sprintf("%v", v["value"])
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			switch t := item.(type) {
			case map[string]any:
				if f, ok := t["filename"].(string); ok {
					parts = append(parts, f)
					continue
				}
				if f, ok := t["name"].(string); ok {
					parts = append(parts, f)
					continue
				}
			default:
				parts = append(parts, fmt.Sprintf("%v", t))
			}
		}
		return strings.Join(parts, ", ")
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}

// firstFieldOfType returns the name of the first field of the given type.
func firstFieldOfType(collection *core.Collection, fieldType string) string {
	if collection == nil {
		return ""
	}
	for _, f := range collection.Fields {
		if f.Type() == fieldType {
			return f.GetName()
		}
	}
	return ""
}

// firstTextField returns the name of the first non-system text field.
func firstTextField(collection *core.Collection) string {
	if collection == nil {
		return ""
	}
	for _, f := range collection.Fields {
		if f.Type() == "text" {
			name := f.GetName()
			if name == core.FieldNameId || name == "perPage" {
				continue
			}
			return name
		}
	}
	return ""
}

// externalNumericValues extracts the numeric values of an external record field.
func externalNumericValues(records []*core.Record, field string) []float64 {
	nums := []float64{}
	for _, rec := range records {
		raw := rec.Get(field)
		switch v := raw.(type) {
		case float64:
			nums = append(nums, v)
		case int:
			nums = append(nums, float64(v))
		case int64:
			nums = append(nums, float64(v))
		case string:
			var f float64
			if _, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &f); err == nil {
				nums = append(nums, f)
			}
		}
	}
	return nums
}

// contains reports whether the target equals any item.
func contains(target string, items []string) bool {
	for _, it := range items {
		if it == target {
			return true
		}
	}
	return false
}