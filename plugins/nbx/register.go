// Package nbx provides the NextBase plugin that wires the app together:
// UI extensions, hooks and CLI commands.
package nbx

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	nbxui "github.com/pocketbase/pocketbase/ui-ext/nbx"
)

// Name is the unique plugin/extension name used as a URL segment.
const Name = "nbx"

// Register binds the NextBase UI extension and hooks to the app instance.
//
// The UI extension is served at /_/extensions/nbx/* and its main.js is
// concatenated into /_/extensions.js.
func Register(app core.App) error {
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Priority: -9999,
		Func: func(se *core.ServeEvent) error {
			se.UIExtensions = append(se.UIExtensions, core.UIExtension{
				Name: Name,
				FS:   nbxui.DistDirFS,
			})

			return se.Next()
		},
	})

	return nil
}
