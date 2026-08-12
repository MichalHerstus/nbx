package jsvm_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pocketbase/pocketbase/tests"
)

func TestRunScriptExpression(t *testing.T) {
	v, err := jsvm.RunScript(nil, "21 * 2;")
	if err != nil {
		t.Fatal(err)
	}
	if v != int64(42) && v != float64(42) && v != 42 {
		t.Fatalf("expected 42, got %#v", v)
	}
}

func TestRunScriptObject(t *testing.T) {
	v, err := jsvm.RunScript(nil, `({a: 1, b: "x"})`)
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %#v", v)
	}
	if obj["a"] != int64(1) && obj["a"] != float64(1) {
		t.Fatalf("unexpected a: %#v", obj["a"])
	}
}

func TestRunScriptError(t *testing.T) {
	_, err := jsvm.RunScript(nil, `throw new Error("boom")`)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunScriptEmpty(t *testing.T) {
	_, err := jsvm.RunScript(nil, "")
	if err == nil {
		t.Fatal("expected error for empty script")
	}
}

func TestRunScriptAppBind(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	v, err := jsvm.RunScript(app, `typeof $app.findCollectionByNameOrId === "function" ? "ok" : "bad"`)
	if err != nil {
		t.Fatal(err)
	}
	if v != "ok" {
		t.Fatalf("expected $app binding, got %#v", v)
	}
}

func TestUnmarshalNbxButtonConfigDefaults(t *testing.T) {
	cfg, err := core.UnmarshalNbxButtonConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Method != "POST" || cfg.TimeoutSec != 30 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}

	cfg2, err := core.UnmarshalNbxButtonConfig(map[string]any{"method": "PUT", "script": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Method != "PUT" || cfg2.Script != "1" {
		t.Fatalf("unexpected config: %+v", cfg2)
	}
}
