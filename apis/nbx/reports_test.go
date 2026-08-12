package nbx_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/apis/nbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/core/pdf"
	"github.com/pocketbase/pocketbase/tests"
)

// setupNbxReport seeds a dashboard + report and returns the report id.
func setupNbxReport(t *testing.T, app *tests.TestApp) string {
	t.Helper()

	dashCol, err := app.FindCollectionByNameOrId(core.CollectionNameDashboards)
	if err != nil {
		t.Fatal(err)
	}

	dashConfig := map[string]any{
		"columns": 12,
		"widgets": []map[string]any{
			{
				"type":      "kpi",
				"title":     "Total count",
				"source":    "demo1",
				"aggregate": "count",
				"span":      3,
			},
			{
				"type":      "kpi",
				"title":     "Sum of numbers",
				"source":    "demo1",
				"aggregate": "sum",
				"field":     "number",
				"span":      3,
			},
			{
				"type":   "table",
				"title":  "Demo rows",
				"source": "demo1",
				"sort":   "created",
				"span":   6,
			},
			{
				"type":  "text",
				"title": "Note",
				"text":  "Generated from the demo collection.",
				"span":  12,
			},
		},
	}

	dash := core.NewRecord(dashCol)
	dash.Set("label", "Demo dashboard")
	dash.Set("config", dashConfig)
	if err := app.Save(dash); err != nil {
		t.Fatal(err)
	}

	reportCol, err := app.FindCollectionByNameOrId(core.CollectionNameReports)
	if err != nil {
		t.Fatal(err)
	}

	report := core.NewRecord(reportCol)
	report.Set("label", "Demo report")
	report.Set("dashboard", dash.Id)
	report.Set("config", map[string]any{"orientation": "portrait", "pageSize": "A4"})
	if err := app.Save(report); err != nil {
		t.Fatal(err)
	}

	return report.Id
}

func superuserAuthToken(t *testing.T, app *tests.TestApp) string {
	t.Helper()
	su, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, err := su.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func newReportApp(t *testing.T) (*tests.TestApp, string, string) {
	t.Helper()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)

	reportID := setupNbxReport(t, app)
	token := superuserAuthToken(t, app)

	return app, reportID, token
}

func TestReportsPdf(t *testing.T) {
	app, reportID, token := newReportApp(t)

	scenario := tests.ApiScenario{
		Name:           "render report pdf",
		Method:         http.MethodGet,
		URL:            "/api/nbx/reports/" + reportID + "/pdf",
		Headers:        map[string]string{"Authorization": token},
		Timeout:        5,
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			"%PDF",
		},
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			return app
		},
		DisableTestAppCleanup: true,
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
			if res.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", res.StatusCode)
			}
			if ct := res.Header.Get("Content-Type"); ct != "application/pdf" {
				t.Fatalf("expected application/pdf, got %q", ct)
			}
			buf := &bytes.Buffer{}
			if _, err := buf.ReadFrom(res.Body); err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) {
				t.Fatalf("expected PDF magic header, got %q", buf.Bytes())
			}
		},
	}
	scenario.Test(t)
}

func TestReportsPdfNotFound(t *testing.T) {
	app, _, token := newReportApp(t)

	scenario := tests.ApiScenario{
		Name:           "missing report",
		Method:         http.MethodGet,
		URL:            "/api/nbx/reports/missing_id/pdf",
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusNotFound,
		ExpectedContent: []string{
			`{"data":{},"message":"The report does not exist.","status":404}`,
		},
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			return app
		},
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestReportsPdfUnauthorized(t *testing.T) {
	app, reportID, _ := newReportApp(t)

	scenario := tests.ApiScenario{
		Name:           "no auth header",
		Method:         http.MethodGet,
		URL:            "/api/nbx/reports/" + reportID + "/pdf",
		ExpectedStatus: http.StatusUnauthorized,
		ExpectedContent: []string{
			`{"data":{},"message":"The request requires valid record authorization token.","status":401}`,
		},
		TestAppFactory: func(t testing.TB) *tests.TestApp {
			return app
		},
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestSetPdfRenderer(t *testing.T) {
	want := &pdf.FpdfRenderer{}
	nbx.SetPdfRenderer(want)
	nbx.SetPdfRenderer(nil) // resets to default
}

func TestDashboardWidgets(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	dashCol, err := app.FindCollectionByNameOrId(core.CollectionNameDashboards)
	if err != nil {
		t.Fatal(err)
	}
	dash := core.NewRecord(dashCol)
	dash.Set("label", "Widgets dashboard")
	dash.Set("config", map[string]any{
		"columns": 12,
		"widgets": []map[string]any{
			{
				"type":      "kpi",
				"title":     "Total",
				"source":    "demo1",
				"aggregate": "count",
				"span":      3,
			},
			{"type": "text", "title": "Note", "text": "hello world", "span": 12},
		},
	})
	if err := app.Save(dash); err != nil {
		t.Fatal(err)
	}

	token := superuserAuthToken(t, app)
	scenario := tests.ApiScenario{
		Name:           "widgets json",
		Method:         http.MethodGet,
		URL:            "/api/nbx/dashboards/" + dash.Id + "/widgets",
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"type":"kpi"`,
			`"count":3`,
			`"type":"text"`,
		},
		TestAppFactory:       func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestAggregateLocal(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	col, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	agg, err := core.AggregateLocal(app, col, "", "number")
	if err != nil {
		t.Fatal(err)
	}
	if agg.Count != 3 {
		t.Fatalf("expected 3 records, got %d", agg.Count)
	}
	// 123456 + 456 + 0 = 123912
	if agg.Sum == nil || *agg.Sum != 123912.0 {
		t.Fatalf("unexpected sum: %v", agg.Sum)
	}

	raw, err := json.Marshal(agg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"count":3`)) {
		t.Fatalf("unexpected JSON: %s", raw)
	}
}

func TestAggregateLocalFiltered(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	col, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	agg, err := core.AggregateLocal(app, col, `number > 400`, "")
	if err != nil {
		t.Fatal(err)
	}
	// records with number > 400: 123456 and 456 -> 2
	if agg.Count != 2 {
		t.Fatalf("expected 2 filtered records, got %d", agg.Count)
	}
}