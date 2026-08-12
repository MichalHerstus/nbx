package nbx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/nbx"
	"github.com/pocketbase/pocketbase/tests"
)

func superuserToken(t *testing.T, app *tests.TestApp) string {
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

// newButtonApp builds a TestApp with the nbx plugin registered (so the button
// routes are bound) and seeds a _buttons record, returning the app, the button
// id and an auth token.
func newButtonApp(t *testing.T, action, target string, config map[string]any) (*tests.TestApp, string, string) {
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

	col, err := app.FindCollectionByNameOrId(core.CollectionNameButtons)
	if err != nil {
		t.Fatal(err)
	}
	record := core.NewRecord(col)
	record.Set("label", "Test button")
	record.Set("action", action)
	record.Set("target", target)
	record.Set("config", config)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	return app, record.Id, superuserToken(t, app)
}

// build serve config so routes are reachable via the ApiScenario router.
func buttonScenario(app *tests.TestApp, url, token string) tests.ApiScenario {
	return tests.ApiScenario{
		Method:               http.MethodPost,
		URL:                  url,
		Headers:              map[string]string{"Authorization": token},
		TestAppFactory:       func(t testing.TB) *tests.TestApp { return app },
		DisableTestAppCleanup: true,
	}
}

func TestRunButtonOpenPage(t *testing.T) {
	app, id, token := newButtonApp(t, core.ButtonActionOpenPage, "#/workspace", nil)

	scenario := buttonScenario(app, "/api/nbx/buttons/"+id+"/run", token)
	scenario.Name = "open_page rejected server-side"
	scenario.ExpectedStatus = http.StatusBadRequest
	scenario.ExpectedContent = []string{`"data":{}`}
	scenario.Test(t)
}

func TestRunButtonRunJS(t *testing.T) {
	app, id, token := newButtonApp(t, core.ButtonActionRunJS, "", map[string]any{
		"script": "21 * 2;",
	})

	scenario := buttonScenario(app, "/api/nbx/buttons/"+id+"/run", token)
	scenario.Name = "run_js returns result"
	scenario.ExpectedStatus = http.StatusOK
	scenario.ExpectedContent = []string{`"action":"run_js"`, `"output":42`}
	scenario.Test(t)
}

func TestRunButtonRunJSUsesAppBinds(t *testing.T) {
	app, id, token := newButtonApp(t, core.ButtonActionRunJS, "", map[string]any{
		"script": `(() => {
			const found = typeof $app.findCollectionByNameOrId === "function";
			return found ? "ok" : "bad";
		})();`,
	})

	scenario := buttonScenario(app, "/api/nbx/buttons/"+id+"/run", token)
	scenario.Name = "run_js has $app binding"
	scenario.ExpectedStatus = http.StatusOK
	scenario.ExpectedContent = []string{`"output":"ok"`}
	scenario.Test(t)
}

func TestRunButtonRunJSError(t *testing.T) {
	app, id, token := newButtonApp(t, core.ButtonActionRunJS, "", map[string]any{
		"script": "throw new Error('boom')",
	})

	scenario := buttonScenario(app, "/api/nbx/buttons/"+id+"/run", token)
	scenario.Name = "run_js propagates script error"
	scenario.ExpectedStatus = http.StatusBadRequest
	scenario.ExpectedContent = []string{`"data":{}`, "Script execution failed"}
	scenario.Test(t)
}

func TestRunButtonWebhook(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := new(strings.Builder)
		_, _ = io.Copy(b, r.Body)
		gotBody = b.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	app, id, token := newButtonApp(t, core.ButtonActionWebhook, server.URL, map[string]any{
		"method": "POST",
		"body":   `{"a":1}`,
	})

	scenario := buttonScenario(app, "/api/nbx/buttons/"+id+"/run", token)
	scenario.Name = "webhook forwards request"
	scenario.ExpectedStatus = http.StatusOK
	scenario.ExpectedContent = []string{`"action":"webhook"`, `"output":"{\"ok\":true}"`, `"message":"200 OK"`}
	scenario.Test(t)

	if gotBody != `{"a":1}` {
		t.Fatalf("expected webhook to receive request body %q, got %q", `{"a":1}`, gotBody)
	}
}

func TestRunButtonNotFound(t *testing.T) {
	app, _, token := newButtonApp(t, core.ButtonActionRunJS, "", map[string]any{"script": "1"})

	scenario := buttonScenario(app, "/api/nbx/buttons/missing/run", token)
	scenario.Name = "missing button"
	scenario.ExpectedStatus = http.StatusNotFound
	scenario.ExpectedContent = []string{`"data":{}`}
	scenario.Test(t)
}

func TestRunButtonUnauthorized(t *testing.T) {
	app, id, _ := newButtonApp(t, core.ButtonActionRunJS, "", map[string]any{"script": "1"})

	scenario := buttonScenario(app, "/api/nbx/buttons/"+id+"/run", "")
	scenario.Name = "no auth header"
	scenario.ExpectedStatus = http.StatusUnauthorized
	scenario.ExpectedContent = []string{`"data":{}`}
	scenario.Test(t)
}
