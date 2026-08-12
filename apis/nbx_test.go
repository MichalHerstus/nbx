package apis_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// superuserAuth is the superuser auth header used in the default test app data.
const superuserAuth = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY"

func TestNbxDatasourcesRoute(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthenticated",
			Method:          http.MethodGet,
			URL:             "/api/nbx/datasources",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/nbx/datasources",
			Headers: map[string]string{
				"Authorization": superuserAuth,
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"types":`,
				`"mysql"`,
				`"postgres"`,
				`"mssql"`,
				`"rest"`,
				`"refresh"`,
				`"secrets":[]`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestRecordCrudExternalREST(t *testing.T) {
	t.Parallel()

	var server *httptest.Server

	scenarios := []tests.ApiScenario{
		{
			Name:   "list external rest datasource",
			Method: http.MethodGet,
			URL:    "/api/collections/rest_products/records?perPage=2",
			Headers: map[string]string{
				"Authorization": superuserAuth,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(`[
						{"id":"1","name":"Apple","category":"Fruit"},
						{"id":"2","name":"Banana","category":"Fruit"},
						{"id":"3","name":"Carrot","category":"Vegetable"}
					]`))
				}))

				col := core.NewBaseCollection("rest_products")
				col.DataSource = core.DataSource{
					Type:    core.DataSourceTypeREST,
					URL:     server.URL,
					Method:  http.MethodGet,
					Refresh: core.DataSourceRefreshManual,
				}
				col.Fields.Add(&core.TextField{Name: "id", PrimaryKey: true})
				col.Fields.Add(&core.TextField{Name: "name"})
				col.Fields.Add(&core.TextField{Name: "category"})

				if err := app.Save(col); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"page":1`,
				`"perPage":2`,
				`"totalItems":3`,
				`"items":[{`,
				`"name":"Apple"`,
			},
			ExpectedEvents: map[string]int{
				"*":                    0,
				"OnRecordsListRequest": 1,
				"OnRecordEnrich":       2,
			},
		},
		{
			Name:   "create on external rest datasource is read-only",
			Method: http.MethodPost,
			URL:    "/api/collections/rest_products/records",
			Headers: map[string]string{
				"Authorization": superuserAuth,
			},
			Body: strings.NewReader(`{"name":"Durian","category":"Fruit"}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				col := core.NewBaseCollection("rest_products")
				col.DataSource = core.DataSource{
					Type:    core.DataSourceTypeREST,
					URL:     "http://127.0.0.1:1/unused",
					Method:  http.MethodGet,
					Refresh: core.DataSourceRefreshManual,
				}
				col.Fields.Add(&core.TextField{Name: "id", PrimaryKey: true})
				col.Fields.Add(&core.TextField{Name: "name"})

				if err := app.Save(col); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"message":"The datasource is read-only."`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "update on external rest datasource is read-only",
			Method: http.MethodPatch,
			URL:    "/api/collections/rest_products/records/1",
			Headers: map[string]string{
				"Authorization": superuserAuth,
			},
			Body: strings.NewReader(`{"name":"Durian"}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				col := core.NewBaseCollection("rest_products")
				col.DataSource = core.DataSource{
					Type:    core.DataSourceTypeREST,
					URL:     "http://127.0.0.1:1/unused",
					Method:  http.MethodGet,
					Refresh: core.DataSourceRefreshManual,
				}
				col.Fields.Add(&core.TextField{Name: "id", PrimaryKey: true})
				col.Fields.Add(&core.TextField{Name: "name"})

				if err := app.Save(col); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"message":"The datasource is read-only."`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "delete on external rest datasource is read-only",
			Method: http.MethodDelete,
			URL:    "/api/collections/rest_products/records/1",
			Headers: map[string]string{
				"Authorization": superuserAuth,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				col := core.NewBaseCollection("rest_products")
				col.DataSource = core.DataSource{
					Type:    core.DataSourceTypeREST,
					URL:     "http://127.0.0.1:1/unused",
					Method:  http.MethodGet,
					Refresh: core.DataSourceRefreshManual,
				}
				col.Fields.Add(&core.TextField{Name: "id", PrimaryKey: true})

				if err := app.Save(col); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"message":"The datasource is read-only."`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}

	if server != nil {
		server.Close()
	}
}
