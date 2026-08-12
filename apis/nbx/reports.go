package nbx

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/core/pdf"
)

// pdfRenderer is the swappable report-to-PDF engine. The HTTP handler is
// identical regardless of the renderer, so an alternative engine can be
// injected at runtime.
var pdfRenderer pdf.Renderer = &pdf.FpdfRenderer{}

// SetPdfRenderer replaces the default PDF renderer (used mostly by tests).
func SetPdfRenderer(r pdf.Renderer) {
	if r == nil {
		r = &pdf.FpdfRenderer{}
	}
	pdfRenderer = r
}

// reportPdfOutput mirrors the pdf.Doc model for JSON-friendly widget rendering
// used by the report endpoint helpers.
type reportWidgetData struct {
	Widget core.NbxWidget `json:"widget"`
	// Kpi holds the KPI aggregate results (nil for non-kpi widgets).
	Kpi *core.NbxAggregate `json:"kpi,omitempty"`
	// Table holds the table widget rows+columns (nil for non-table widgets).
	Table *reportTable `json:"table,omitempty"`
	// Notes holds the parsed text widget lines (nil for non-text widgets).
	Notes []string `json:"notes,omitempty"`
	// Error holds a non-critical widget evaluation error message.
	Error string `json:"error,omitempty"`
}

type reportTable struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

// dashboardWidgets returns a handler evaluating a _dashboards record widgets
// into a JSON payload for the dashboard SPA (KPI values, table rows, notes).
func dashboardWidgets(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		dashboard, err := app.FindRecordById(core.CollectionNameDashboards, e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("The dashboard does not exist.", err)
		}

		dashConfig, err := core.UnmarshalNbxDashboardConfig(dashboard.Get("config"))
		if err != nil {
			return e.BadRequestError("Invalid dashboard config.", err)
		}

		data, err := evalWidgets(e, app, dashConfig, nil)
		if err != nil {
			return e.InternalServerError("Failed to evaluate the dashboard widgets.", err)
		}

		return e.JSON(http.StatusOK, map[string]any{
			"id":    dashboard.Id,
			"label": dashboard.GetString("label"),
			"data":  data,
		})
	}
}

// reportsPdf returns a handler rendering a _reports record to a PDF document.
//
// The report is bound to a _dashboards record; the widget list is resolved
// from the dashboard config and each widget is evaluated server-side
// (aggregations via the records/datasource query path and table rows from the
// collection list query).
func reportsPdf(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		report, err := app.FindRecordById(core.CollectionNameReports, e.Request.PathValue("id"))
		if err != nil {
			return e.NotFoundError("The report does not exist.", err)
		}

		config, err := core.UnmarshalNbxReportConfig(report.Get("config"))
		if err != nil {
			return e.BadRequestError("Invalid report config.", err)
		}

		dashboard, err := app.FindRecordById(core.CollectionNameDashboards, report.GetString("dashboard"))
		if err != nil {
			return e.BadRequestError("The report dashboard does not exist.", err)
		}

		dashConfig, err := core.UnmarshalNbxDashboardConfig(dashboard.Get("config"))
		if err != nil {
			return e.BadRequestError("Invalid dashboard config.", err)
		}

		data, err := evalWidgets(e, app, dashConfig, config)
		if err != nil {
			return e.InternalServerError("Failed to evaluate the report widgets.", err)
		}

		doc := pdf.Doc{
			Title:    config.Title,
			Subtitle: strings.TrimSpace(report.GetString("label")) + " — " + strings.TrimSpace(dashboard.GetString("label")),
		}
		for _, w := range data {
			switch w.Widget.Type {
			case core.WidgetTypeText:
				doc.Notes = append(doc.Notes, w.Notes...)
			case core.WidgetTypeKPI:
				if w.Kpi != nil {
					doc.Stats = append(doc.Stats, pdf.Metric{Label: w.Widget.Title, Value: fmt.Sprintf("%d", w.Kpi.Count)})
				}
			case core.WidgetTypeTable:
				if w.Table != nil {
					doc.Tables = append(doc.Tables, pdf.Table{Title: w.Widget.Title, Columns: w.Table.Columns, Rows: w.Table.Rows})
				}
			}
		}

		bytes, err := pdfRenderer.Render(doc)
		if err != nil {
			return e.InternalServerError("Failed to render the report pdf.", err)
		}

		e.Response.Header().Set("Content-Disposition", `inline; filename="report.pdf"`)
		return e.Blob(http.StatusOK, pdfRenderer.Mime(), bytes)
	}
}

// evalWidgets evaluates every dashboard widget server-side into a renderable
// payload. It never fails the whole report for a single widget error - the
// error is captured per widget.
func evalWidgets(
	e *core.RequestEvent,
	app core.App,
	dashConfig *core.NbxDashboardConfig,
	reportConfig *core.NbxReportConfig,
) ([]reportWidgetData, error) {
	result := make([]reportWidgetData, 0, len(dashConfig.Widgets))

	for _, widget := range dashConfig.Widgets {
		if reportConfig != nil && len(reportConfig.IncludeWidgets) > 0 && !contains(widget.Type, reportConfig.IncludeWidgets) {
			continue
		}

		data := reportWidgetData{Widget: widget}
		var err error

		switch widget.Type {
		case core.WidgetTypeText:
			data.Notes = strings.Split(widget.Text, "\n")
		case core.WidgetTypeKPI:
			data.Kpi, err = widgetAggregate(e, app, widget)
		case core.WidgetTypeTable, core.WidgetTypeChart:
			data.Table, err = widgetTable(e, app, widget)
		case core.WidgetTypeMap:
			data.Table, err = widgetMapTable(e, app, widget)
		}

		if err != nil {
			data.Error = err.Error()
		}
		result = append(result, data)
	}

	return result, nil
}

// widgetAggregate computes the KPI aggregation for a widget.
func widgetAggregate(e *core.RequestEvent, app core.App, widget core.NbxWidget) (*core.NbxAggregate, error) {
	collection, err := resolveWidgetCollection(app, widget)
	if err != nil {
		return nil, err
	}

	if !collection.GetDataSource().IsLocal() {
		return externalAggregate(app, collection, widget)
	}
	return core.AggregateLocal(app, collection, widget.Filter, widget.Field)
}

// widgetTable loads the table widget rows/columns from the collection list
// query (local collections honour the configured sort/filter/perPage).
func widgetTable(e *core.RequestEvent, app core.App, widget core.NbxWidget) (*reportTable, error) {
	collection, err := resolveWidgetCollection(app, widget)
	if err != nil {
		return nil, err
	}

	requestInfo, err := e.RequestInfo()
	if err != nil {
		return nil, err
	}

	fields := structFieldNames(collection)
	if len(fields) == 0 {
		return &reportTable{Columns: []string{}, Rows: [][]string{}}, nil
	}

	perPage := widget.PerPage
	if perPage <= 0 {
		perPage = 50
	}

	records, err := loadWidgetRecords(e, app, collection, widget, requestInfo, perPage)
	if err != nil {
		return nil, err
	}

	table := &reportTable{Columns: fields, Rows: make([][]string, 0, len(records))}
	for _, rec := range records {
		row := make([]string, 0, len(fields))
		for _, f := range fields {
			row = append(row, formatWidgetValue(rec.Get(f), f))
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

// widgetMapTable flattens geo locations of a map widget into a small table
// (coordinate/title pairs) suitable for the printed report.
func widgetMapTable(e *core.RequestEvent, app core.App, widget core.NbxWidget) (*reportTable, error) {
	collection, err := resolveWidgetCollection(app, widget)
	if err != nil {
		return nil, err
	}

	requestInfo, err := e.RequestInfo()
	if err != nil {
		return nil, err
	}

	perPage := widget.PerPage
	if perPage <= 0 {
		perPage = 2000
	}

	records, err := loadWidgetRecords(e, app, collection, widget, requestInfo, perPage)
	if err != nil {
		return nil, err
	}

	geoField := widget.Field
	if geoField == "" {
		geoField = firstFieldOfType(collection, "geoPoint")
	}
	titleField := widget.Field2
	if titleField == "" {
		titleField = firstTextField(collection)
	}

	table := &reportTable{Columns: []string{"Title", "Latitude", "Longitude"}}
	for _, rec := range records {
		gp := rec.GetGeoPoint(geoField)
		if gp.Lat == 0 && gp.Lon == 0 {
			continue
		}
		lat := gp.Lat
		lon := gp.Lon
		title := ""
		if titleField != "" {
			if v := rec.GetString(titleField); v != "" {
				title = v
			}
		}
		if title == "" {
			title = rec.GetString(core.FieldNameId)
		}
		table.Rows = append(table.Rows, []string{title, fmt.Sprintf("%.5f", lat), fmt.Sprintf("%.5f", lon)})
	}
	return table, nil
}

// resolveWidgetCollection finds the collection referenced by a widget source.
func resolveWidgetCollection(app core.App, widget core.NbxWidget) (*core.Collection, error) {
	source := strings.TrimSpace(widget.Source)
	if source == "" {
		return nil, fmt.Errorf("widget has no source collection")
	}
	collection, err := app.FindCachedCollectionByNameOrId(source)
	if err != nil || collection == nil {
		return nil, fmt.Errorf("source collection %q not found", source)
	}
	return collection, nil
}

// externalAggregate computes the KPI aggregation for external datasources by
// fetching all matching rows through the datasource registry.
func externalAggregate(app core.App, collection *core.Collection, widget core.NbxWidget) (*core.NbxAggregate, error) {
	cred := datasourceCredential(app, collection.GetDataSource())

	result, err := datasourceRegistry.List(collection, cred, 1, maxAggregateRows, widget.Sort, widget.Filter)
	if err != nil {
		return nil, err
	}

	records, _ := result.Items.([]*core.Record)
	out := &core.NbxAggregate{Count: result.TotalItems}

	if widget.Field == "" {
		return out, nil
	}

	nums := externalNumericValues(records, widget.Field)
	if len(nums) > 0 {
		var sum float64
		min := nums[0]
		max := nums[0]
		for _, n := range nums {
			sum += n
			if n < min {
				min = n
			}
			if n > max {
				max = n
			}
		}
		avg := sum / float64(len(nums))
		out.Sum, out.Avg, out.Min, out.Max = &sum, &avg, &min, &max
	}

	return out, nil
}