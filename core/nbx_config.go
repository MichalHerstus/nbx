package core

import (
	"encoding/json"

	validation "github.com/pocketbase/ozzo-validation/v4"
)

// NextBase dashboard/report widget types.
const (
	WidgetTypeKPI    = "kpi"
	WidgetTypeTable  = "table"
	WidgetTypeChart  = "chart"
	WidgetTypeText   = "text"
	WidgetTypeMap    = "map"
	WidgetTypeSpacer = "spacer"
)

// KPI aggregate operations.
const (
	AggregateCount = "count"
	AggregateSum   = "sum"
	AggregateAvg   = "avg"
	AggregateMin   = "min"
	AggregateMax   = "max"
)

// NbxWidget describes a single dashboard cell in the layout grid.
//
// The source field references a collection by id or name. Depending on the
// widget type, the remaining fields configure how its data is fetched and
// rendered (see plan F3).
type NbxWidget struct {
	// Type is "kpi", "table", "chart", "text" or "map".
	Type string `json:"type"`

	// Title is the optional widget heading.
	Title string `json:"title"`

	// Source is the collection id/name the widget reads from.
	Source string `json:"source"`

	// Filter is an optional search.FilterData filter expression.
	Filter string `json:"filter"`

	// Sort is an optional comma separated list of fields to sort by.
	Sort string `json:"sort"`

	// Aggregate is the KPI operation ("count", "sum", "avg", "min", "max").
	Aggregate string `json:"aggregate"`

	// Field is the value field for KPI/chart widgets and the geoPoint field
	// for map widgets.
	Field string `json:"field"`

	// Field2 is an optional second value field (currently used by chart y-axis).
	Field2 string `json:"field2"`

	// Text is the static content of a text widget.
	Text string `json:"text"`

	// PerPage limits the number of rows rendered for table/chart widgets.
	PerPage int `json:"perPage"`

	// Span is the widget width in the 12-column layout grid (default 4).
	Span int `json:"span"`

	// Hidden hides the widget from the dashboard canvas (still shows on reports).
	Hidden bool `json:"hidden"`

	// Extra carries renderer-specific options (eg. map cluster, chart colors).
	Extra map[string]any `json:"extra,omitempty"`
}

// Validate implements the [validation.Validatable] interface.
func (w NbxWidget) Validate() error {
	return validation.ValidateStruct(&w,
		validation.Field(&w.Type, validation.In(
			WidgetTypeKPI,
			WidgetTypeTable,
			WidgetTypeChart,
			WidgetTypeText,
			WidgetTypeMap,
			WidgetTypeSpacer,
		)),
		validation.Field(&w.Aggregate, validation.In(
			AggregateCount, AggregateSum, AggregateAvg, AggregateMin, AggregateMax,
		)),
		validation.Field(&w.Span, validation.Min(0), validation.Max(12)),
		validation.Field(&w.PerPage, validation.Min(0)),
	)
}

// NbxDashboardConfig is the parsed content of the _dashboards "config" field.
type NbxDashboardConfig struct {
	// Widgets is the ordered list of dashboard cells.
	Widgets []NbxWidget `json:"widgets"`

	// Columns is the optional number of layout columns (default 12).
	Columns int `json:"columns"`
}

// Validate implements the [validation.Validatable] interface.
func (c NbxDashboardConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Columns, validation.Min(0), validation.Max(12)),
		validation.Field(&c.Widgets),
	)
}

// Default returns the default dashboard config.
func (c *NbxDashboardConfig) Default() {
	if c == nil {
		*c = NbxDashboardConfig{Columns: 12}
		return
	}
	if c.Columns == 0 {
		c.Columns = 12
	}
	if c.Widgets == nil {
		c.Widgets = []NbxWidget{}
	}
}

// NbxReportConfig is the parsed content of the _reports "config" field.
type NbxReportConfig struct {
	// Orientation is "portrait" (default) or "landscape".
	Orientation string `json:"orientation"`

	// PageSize is "A4" (default) or "letter".
	PageSize string `json:"pageSize"`

	// Title is an optional override for the report headline.
	Title string `json:"title"`

	// IncludeWidgets optionally restricts which dashboard widgets render.
	// Empty means include all.
	IncludeWidgets []string `json:"includeWidgets"`
}

// Validate implements the [validation.Validatable] interface.
func (c NbxReportConfig) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Orientation, validation.In("", "portrait", "landscape")),
		validation.Field(&c.PageSize, validation.In("", "A4", "letter")),
	)
}

// Default returns the default report config.
func (c *NbxReportConfig) Default() {
	if c == nil {
		*c = NbxReportConfig{Orientation: "portrait", PageSize: "A4"}
		return
	}
	if c.Orientation == "" {
		c.Orientation = "portrait"
	}
	if c.PageSize == "" {
		c.PageSize = "A4"
	}
}

// UnmarshalNbxDashboardConfig parses a dashboard config JSON value into a
// NbxDashboardConfig with defaults applied.
func UnmarshalNbxDashboardConfig(raw any) (*NbxDashboardConfig, error) {
	var out NbxDashboardConfig
	out.Default()
	if raw == nil {
		return &out, nil
	}

	switch v := raw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, err
		}
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, err
		}
	}

	if out.Columns == 0 {
		out.Columns = 12
	}
	if out.Widgets == nil {
		out.Widgets = []NbxWidget{}
	}

	return &out, nil
}

// UnmarshalNbxReportConfig parses a report config JSON value into a
// NbxReportConfig with defaults applied.
func UnmarshalNbxReportConfig(raw any) (*NbxReportConfig, error) {
	var out NbxReportConfig
	out.Default()
	if raw == nil {
		return &out, nil
	}

	switch v := raw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, err
		}
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, err
		}
	}

	out.Default()
	return &out, nil
}