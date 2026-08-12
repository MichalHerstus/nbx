package nbx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/search"
)

// uiViewOutput is a view ("_views" record) surfaced on /ui.
type uiViewOutput struct {
	Id     string `json:"id"`
	Label  string `json:"label"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Config any    `json:"config"`
}

// uiCollectionOutput is a user-facing (authenticated) view of a collection
// plus its defined views. Secrets/system internals are not exposed.
type uiCollectionOutput struct {
	Id     string         `json:"id"`
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	Fields []string       `json:"fields"`
	Views  []uiViewOutput `json:"views"`
}

// uiCollections returns the handler that lists, for the current authenticated
// user, the accessible collections and their defined views (drives the /ui nav).
func uiCollections(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		requestInfo, err := e.RequestInfo()
		if err != nil {
			return e.BadRequestError("", err)
		}

		views, err := loadViewsMap(app)
		if err != nil {
			return e.InternalServerError("Failed to load views.", err)
		}

		all, err := app.FindAllCollections()
		if err != nil {
			return e.InternalServerError("Failed to load collections.", err)
		}

		result := []uiCollectionOutput{}
		for _, col := range all {
			if !canListCollection(app, requestInfo, col) {
				continue
			}

			out := uiCollectionOutput{
				Id:   col.Id,
				Name: col.Name,
				Type: col.Type,
			}
			for _, f := range col.Fields {
				name := f.GetName()
				if name == core.FieldNameId || name == "created" || name == "updated" {
					continue
				}
				out.Fields = append(out.Fields, name)
			}

			if colViews, ok := views[col.Id]; ok {
				out.Views = colViews
			}
			if colViews, ok := views[col.Name]; ok && len(out.Views) == 0 {
				out.Views = colViews
			}

			result = append(result, out)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"collections": result,
		})
	}
}

// uiView returns the handler that serves a single view for the current user:
// the source collection schema + view config + filtered records.
func uiView(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		view, err := app.FindRecordById(core.CollectionNameViews, e.Request.PathValue("id"))
		if err != nil || view == nil {
			return e.NotFoundError("The view does not exist.", err)
		}

		collectionIdOrName := strings.TrimSpace(view.GetString("sourceCollection"))
		if collectionIdOrName == "" {
			return e.BadRequestError("The view has no source collection.", nil)
		}

		collection, err := app.FindCachedCollectionByNameOrId(collectionIdOrName)
		if err != nil || collection == nil {
			return e.NotFoundError("The view source collection does not exist.", err)
		}

		requestInfo, err := e.RequestInfo()
		if err != nil {
			return e.BadRequestError("", err)
		}

		viewType := strings.TrimSpace(view.GetString("type"))
		if viewType == "" {
			viewType = "grid"
		}

		config := nbxJSONConfig(view.Get("config"))

		records, total, err := listRecordsForUser(e, app, collection, requestInfo, config)
		if err != nil {
			return e.BadRequestError("Failed to load records.", err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"id":    view.Id,
			"label": view.GetString("label"),
			"view": map[string]any{
				"type":             viewType,
				"sourceCollection": collectionIdOrName,
				"config":           view.Get("config"),
			},
			"collection": collection,
			"records":    records,
			"total":      total,
		})
	}
}

// loadViewsMap loads all "_views" records grouped by their source collection.
func loadViewsMap(app core.App) (map[string][]uiViewOutput, error) {
	viewsCol, err := app.FindCollectionByNameOrId(core.CollectionNameViews)
	if err != nil {
		return nil, err
	}

	records := []*core.Record{}
	query := app.RecordQuery(viewsCol)
	if err := query.All(&records); err != nil {
		return nil, err
	}

	result := map[string][]uiViewOutput{}
	for _, r := range records {
		source := strings.TrimSpace(r.GetString("sourceCollection"))
		if source == "" {
			continue
		}
		result[source] = append(result[source], uiViewOutput{
			Id:     r.Id,
			Label:  r.GetString("label"),
			Type:   r.GetString("type"),
			Source: source,
			Config: r.Get("config"),
		})
	}

	return result, nil
}

// canListCollection reports whether the current request can list records of
// the provided collection (honouring ListRule - nil means superuser only).
func canListCollection(app core.App, requestInfo *core.RequestInfo, collection *core.Collection) bool {
	if collection == nil {
		return false
	}

	if requestInfo.HasSuperuserAuth() {
		return true
	}

	if collection.ListRule == nil {
		return false
	}
	if *collection.ListRule == "" {
		return true
	}

	resolver := core.NewRecordFieldResolver(app, collection, requestInfo, true)
	resolver.SetAllowHiddenFields(false)

	expr, err := search.FilterData(*collection.ListRule).BuildExpr(resolver)
	if err != nil {
		return false
	}

	query := app.RecordQuery(collection).Select("(1)").AndWhere(expr)
	if err := resolver.UpdateQuery(query); err != nil {
		return false
	}

	var exists int
	if err := query.Limit(1).Row(&exists); err != nil {
		return false
	}

	return exists > 0
}

// listRecordsForUser returns the records of a collection for the current user,
// applying the standard ListRule (when not superuser) and the view config
// filter/sort/perPage. External datasources are read through the datasource
// registry (read-only).
func listRecordsForUser(
	e *core.RequestEvent,
	app core.App,
	collection *core.Collection,
	requestInfo *core.RequestInfo,
	config map[string]any,
) ([]*core.Record, int, error) {
	perPage := 100
	if v, ok := config["perPage"].(float64); ok && v > 0 {
		perPage = int(v)
	}
	filter, _ := config["filter"].(string)
	sort, _ := config["sort"].(string)

	if !collection.GetDataSource().IsLocal() {
		cred := datasourceCredential(app, collection.GetDataSource())
		result, err := datasourceRegistry.List(collection, cred, 1, perPage, sort, filter)
		if err != nil {
			return nil, 0, err
		}
		records, _ := result.Items.([]*core.Record)
		return records, result.TotalItems, nil
	}

	return listLocalRecords(app, collection, requestInfo, filter, sort, perPage)
}

// listLocalRecords lists records of a SQLite-backed collection honouring the
// ListRule and the provided filter/sort.
func listLocalRecords(
	app core.App,
	collection *core.Collection,
	requestInfo *core.RequestInfo,
	filter string,
	sort string,
	perPage int,
) ([]*core.Record, int, error) {
	// nil ListRule means "superusers only" - mirror the standard records API
	if !requestInfo.HasSuperuserAuth() && collection.ListRule == nil {
		return nil, 0, fmt.Errorf("listing records of %q is not allowed", collection.Name)
	}

	query := app.RecordQuery(collection)
	resolver := core.NewRecordFieldResolver(app, collection, requestInfo, true)
	resolver.SetAllowHiddenFields(requestInfo.HasSuperuserAuth())

	if !requestInfo.HasSuperuserAuth() && collection.ListRule != nil && *collection.ListRule != "" {
		expr, err := search.FilterData(*collection.ListRule).BuildExpr(resolver)
		if err != nil {
			return nil, 0, err
		}
		query.AndWhere(expr)
	}

	filter = strings.TrimSpace(filter)
	if filter != "" {
		expr, err := search.FilterData(filter).BuildExpr(resolver)
		if err != nil {
			return nil, 0, err
		}
		query.AndWhere(expr)
	}

	if err := resolver.UpdateQuery(query); err != nil {
		return nil, 0, err
	}

	// total
	total := 0
	if requestInfo.HasSuperuserAuth() {
		total, _ = recordCount(app, collection, filter)
	} else if collection.ListRule != nil && *collection.ListRule == "" {
		total, _ = recordCount(app, collection, filter)
	} else {
		// for non-empty list rule reuse the query count (approximate but safe)
		countQuery := app.RecordQuery(collection).Select("COUNT(*)")
		cresolver := core.NewRecordFieldResolver(app, collection, requestInfo, true)
		if !requestInfo.HasSuperuserAuth() && collection.ListRule != nil && *collection.ListRule != "" {
			if cexpr, cerr := search.FilterData(*collection.ListRule).BuildExpr(cresolver); cerr == nil {
				countQuery.AndWhere(cexpr)
			}
		}
		if strings.TrimSpace(filter) != "" {
			if cexpr, cerr := search.FilterData(filter).BuildExpr(cresolver); cerr == nil {
				countQuery.AndWhere(cexpr)
			}
		}
		var raw []int64
		if err := countQuery.Limit(1).Column(&raw); err == nil && len(raw) > 0 {
			total = int(raw[0])
		}
	}

	if sort = strings.TrimSpace(sort); sort != "" {
		query.OrderBy(widgetSortToColumns(collection, sort)...)
	}
	if perPage > 0 {
		query.Limit(int64(perPage))
	}

	records := []*core.Record{}
	if err := query.All(&records); err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// recordCount returns the total record count for the given local collection
// and optional filter (public/nil-rule path).
func recordCount(app core.App, collection *core.Collection, filter string) (int, error) {
	query := app.RecordQuery(collection).Select("COUNT(*)")
	filter = strings.TrimSpace(filter)
	if filter != "" {
		info := &core.RequestInfo{Method: "GET"}
		resolver := core.NewRecordFieldResolver(app, collection, info, true)
		if expr, err := search.FilterData(filter).BuildExpr(resolver); err == nil {
			query.AndWhere(expr)
		}
	}
	var raw []int64
	if err := query.Limit(1).Column(&raw); err != nil {
		return 0, err
	}
	if len(raw) == 0 {
		return 0, nil
	}
	return int(raw[0]), nil
}

// nbxJSONConfig normalizes a value into a map for view config access.
func nbxJSONConfig(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case string:
		out := map[string]any{}
		if err := json.Unmarshal([]byte(v), &out); err == nil {
			return out
		}
	}
	return map[string]any{}
}
