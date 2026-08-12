package nbx_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/nbx"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRegisterAddsUIExtension(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	if err := nbx.Register(app); err != nil {
		t.Fatal(err)
	}

	serveEvent := new(core.ServeEvent)
	serveEvent.App = app
	if err := app.OnServe().Trigger(serveEvent, func(e *core.ServeEvent) error {
		return e.Next()
	}); err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, ext := range serveEvent.UIExtensions {
		if ext.Name == nbx.Name && ext.FS != nil {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected the nbx UI extension to be registered")
	}
}
