package jsvm

import (
	"errors"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/process"
	"github.com/dop251/goja_nodejs/require"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/template"
)

// runScriptTimeout bounds a single script execution.
const runScriptTimeout = 10 * time.Second

// RunScript executes the provided JavaScript source in a fresh goja runtime
// that is bound with the same standard app modules available to the JS app
// hooks (e.g. $app, $dbx, $http, $security, $os, ...).
//
// It is mainly used by the NextBase "run_js" buttons (plan F4) to reuse the
// existing PB JSVM bindings. The result is the value of the last evaluated
// expression of the script (or nil when a statement-only script is passed).
func RunScript(app core.App, source string) (any, error) {
	if source == "" {
		return nil, errors.New("empty script source")
	}

	registry := new(require.Registry) // can be shared across runtimes
	templateRegistry := template.NewRegistry()

	vm := goja.New()
	registry.Enable(vm)
	console.Enable(vm)
	process.Enable(vm)
	buffer.Enable(vm)

	BindCore(vm)
	BindDbx(vm)
	BindSecurity(vm)
	BindOS(vm)
	BindFilepath(vm)
	BindHTTP(vm)
	BindFilesystem(vm)
	BindForms(vm)
	BindMails(vm)
	BindApis(vm)

	vm.Set("$app", app)
	vm.Set("$template", templateRegistry)

	// enforce a hard stop on runaway scripts
	timer := time.AfterFunc(runScriptTimeout, func() {
		vm.Interrupt("script execution timed out")
	})
	defer timer.Stop()

	value, err := vm.RunString(source)
	if err != nil {
		// clear the interruption flag so the runtime can be reused if ever needed
		vm.ClearInterrupt()
		return nil, normalizeException(err)
	}

	return normalizeRunScriptResult(value), nil
}

// normalizeRunScriptResult converts a goja value into a JSON-serializable Go
// value (recursively exporting objects/arrays).
func normalizeRunScriptResult(v goja.Value) any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	return v.Export()
}
