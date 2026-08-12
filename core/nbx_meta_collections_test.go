package core_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestNbxMetaCollections(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	expected := []string{
		core.CollectionNameDatasources,
		core.CollectionNameViews,
		core.CollectionNameDashboards,
		core.CollectionNameReports,
		core.CollectionNameButtons,
		core.CollectionNamePreferences,
	}

	for _, name := range expected {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("expected meta collection %q to exist: %v", name, err)
		}

		if !col.System {
			t.Fatalf("expected meta collection %q to be a system collection", name)
		}

		if !col.IsBase() {
			t.Fatalf("expected meta collection %q to be a base collection", name)
		}
	}
}

func TestNbxMetaCollectionsCRUD(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	col, err := app.FindCollectionByNameOrId(core.CollectionNameViews)
	if err != nil {
		t.Fatal(err)
	}

	exists := false
	for _, f := range col.Fields {
		if f.GetName() == "config" {
			exists = true
			break
		}
	}
	if !exists {
		t.Fatal("expected the _views collection to have a config field")
	}
}
