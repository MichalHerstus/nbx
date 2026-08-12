package core_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestUnmarshalNbxDashboardConfigDefaults(t *testing.T) {
	cfg, err := core.UnmarshalNbxDashboardConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Columns != 12 {
		t.Fatalf("expected default 12 columns, got %d", cfg.Columns)
	}
	if cfg.Widgets == nil || len(cfg.Widgets) != 0 {
		t.Fatalf("expected empty widgets, got %+v", cfg.Widgets)
	}
}

func TestUnmarshalNbxDashboardConfigWidgets(t *testing.T) {
	raw := map[string]any{
		"columns": 6,
		"widgets": []map[string]any{
			{"type": "kpi", "title": "Count", "source": "demo1", "aggregate": "sum", "span": 3},
			{"type": "text", "title": "Note", "text": "hello"},
		},
	}
	cfg, err := core.UnmarshalNbxDashboardConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Columns != 6 {
		t.Fatalf("expected 6 columns, got %d", cfg.Columns)
	}
	if len(cfg.Widgets) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(cfg.Widgets))
	}
	if cfg.Widgets[0].Type != core.WidgetTypeKPI || cfg.Widgets[0].Span != 3 {
		t.Fatalf("unexpected first widget: %+v", cfg.Widgets[0])
	}
	if cfg.Widgets[1].Text != "hello" {
		t.Fatalf("unexpected second widget: %+v", cfg.Widgets[1])
	}
}

func TestUnmarshalNbxDashboardConfigInvalidJSON(t *testing.T) {
	_, err := core.UnmarshalNbxDashboardConfig("{invalid json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON string")
	}
}

func TestNbxWidgetValidate(t *testing.T) {
	w := core.NbxWidget{Type: "bogus"}
	if err := w.Validate(); err == nil {
		t.Fatal("expected validation error for invalid widget type")
	}

	w2 := core.NbxWidget{Type: core.WidgetTypeKPI, Aggregate: "sum"}
	if err := w2.Validate(); err != nil {
		t.Fatalf("expected valid widget to pass: %v", err)
	}
}

func TestNbxReportConfigDefaults(t *testing.T) {
	cfg, err := core.UnmarshalNbxReportConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Orientation != "portrait" || cfg.PageSize != "A4" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	cfg2, err := core.UnmarshalNbxReportConfig(map[string]any{"orientation": "landscape"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Orientation != "landscape" {
		t.Fatalf("expected landscape orientation")
	}
}