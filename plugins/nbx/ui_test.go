package nbx_test

import (
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/nbx"
	"github.com/pocketbase/pocketbase/tests"
)

// newUIApp builds a TestApp with the nbx plugin registered (so the /api/nbx/ui
// routes + /ui static mount are bound).
func newUIApp(t *testing.T) *tests.TestApp {
	t.Helper()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Cleanup)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		return e.Next()
	})
	if err := nbx.Register(app); err != nil {
		t.Fatal(err)
	}

	return app
}

func TestUICollectionsSuperuser(t *testing.T) {
	app := newUIApp(t)
	token := superuserToken(t, app)

	scenario := tests.ApiScenario{
		Name:           "superuser sees collections",
		Method:         http.MethodGet,
		URL:            "/api/nbx/ui/collections",
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"collections":`,
			`"name":"demo1"`,
		},
		TestAppFactory:       func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestUICollectionsUnauthorized(t *testing.T) {
	app := newUIApp(t)

	scenario := tests.ApiScenario{
		Name:           "no auth header",
		Method:         http.MethodGet,
		URL:            "/api/nbx/ui/collections",
		ExpectedStatus: http.StatusUnauthorized,
		ExpectedContent: []string{
			`{"data":{},"message":"The request requires valid record authorization token.","status":401}`,
		},
		TestAppFactory: func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestUIViewSuperuser(t *testing.T) {
	app := newUIApp(t)
	token := superuserToken(t, app)

	viewsCol, err := app.FindCollectionByNameOrId(core.CollectionNameViews)
	if err != nil {
		t.Fatal(err)
	}
	view := core.NewRecord(viewsCol)
	view.Set("label", "Demo grid")
	view.Set("sourceCollection", "demo1")
	view.Set("type", "grid")
	view.Set("config", map[string]any{"perPage": 50})
	if err := app.Save(view); err != nil {
		t.Fatal(err)
	}

	scenario := tests.ApiScenario{
		Name:           "view returns schema + records",
		Method:         http.MethodGet,
		URL:            "/api/nbx/ui/view/" + view.Id,
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"label":"Demo grid"`,
			`"name":"demo1"`,
			`"total":3`,
		},
		TestAppFactory:       func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestUIViewNotFound(t *testing.T) {
	app := newUIApp(t)
	token := superuserToken(t, app)

	scenario := tests.ApiScenario{
		Name:           "missing view",
		Method:         http.MethodGet,
		URL:            "/api/nbx/ui/view/missing",
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusNotFound,
		ExpectedContent: []string{
			`{"data":{},"message":"The view does not exist.","status":404}`,
		},
		TestAppFactory: func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestUIStaticMount(t *testing.T) {
	app := newUIApp(t)

	// the /ui mount should serve the bundled SPA (index.html)
	scenario := tests.ApiScenario{
		Name:           "/ui serves admin SPA",
		Method:         http.MethodGet,
		URL:            "/ui/",
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{"<!doctype html>", "shablon"},
		TestAppFactory: func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestUIViewRegularUserNoAccess(t *testing.T) {
	app := newUIApp(t)

	viewsCol, err := app.FindCollectionByNameOrId(core.CollectionNameViews)
	if err != nil {
		t.Fatal(err)
	}
	view := core.NewRecord(viewsCol)
	view.Set("label", "Private grid")
	view.Set("sourceCollection", "demo1") // nil ListRule => superuser-only
	view.Set("type", "grid")
	view.Set("config", map[string]any{"perPage": 50})
	if err := app.Save(view); err != nil {
		t.Fatal(err)
	}

	user, err := app.FindAuthRecordByEmail("users", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}

	// A regular user cannot read records of a collection with nil ListRule
	// (superuser-only), matching the standard records API behavior.
	scenario := tests.ApiScenario{
		Name:           "regular user lacks access to demo1",
		Method:         http.MethodGet,
		URL:            "/api/nbx/ui/view/" + view.Id,
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusBadRequest,
		ExpectedContent: []string{
			`"data":{}`,
			"Failed to load records",
		},
		TestAppFactory:       func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

func TestUICollectionsRegularUserRules(t *testing.T) {	app := newUIApp(t)

	// a regular "users" auth record; demo1 has nil ListRule (superuser-only),
	// demo5 has a rule that may allow some, _mfas is owner-based.
	user, err := app.FindAuthRecordByEmail("users", "test@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}

	scenario := tests.ApiScenario{
		Name:           "regular user sees only rule-accessible collections",
		Method:         http.MethodGet,
		URL:            "/api/nbx/ui/collections",
		Headers:        map[string]string{"Authorization": token},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"collections":`,
		},
		NotExpectedContent: []string{
			`"name":"demo1"`, // nil list rule => not shown to regular users
		},
		TestAppFactory:       func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
	scenario.Test(t)
}

