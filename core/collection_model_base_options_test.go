package core_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestCollectionGetDataSourceDefaults(t *testing.T) {
	t.Parallel()

	col := core.NewBaseCollection("test")

	ds := col.GetDataSource()
	if !ds.IsLocal() {
		t.Fatalf("expected default local datasource, got %+v", ds)
	}

	// non-base collections always return the default local datasource
	view := core.NewViewCollection("test_view")
	viewDs := view.GetDataSource()
	if !viewDs.IsLocal() {
		t.Fatalf("expected view collection to fallback to local datasource")
	}
}

func TestCollectionDataSourceRoundtrip(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	col := core.NewBaseCollection("test_ds")
	col.DataSource = core.DataSource{
		Type:          core.DataSourceTypePostgres,
		CredentialRef: "pg_creds",
		Host:          "db.example.com",
		Port:          5432,
		DB:            "orders",
		Table:         "items",
		Refresh:       core.DataSourceRefreshRealtime,
	}

	if err := app.Save(col); err != nil {
		t.Fatal(err)
	}

	// verify the persisted collection options contain the datasource
	loaded, err := app.FindCollectionByNameOrId("test_ds")
	if err != nil {
		t.Fatal(err)
	}

	ds := loaded.GetDataSource()
	if ds.Type != core.DataSourceTypePostgres || ds.CredentialRef != "pg_creds" ||
		ds.Host != "db.example.com" || ds.Port != 5432 || ds.DB != "orders" ||
		ds.Table != "items" || ds.Refresh != core.DataSourceRefreshRealtime {
		t.Fatalf("persisted datasource mismatch: %+v", ds)
	}

	// verify raw options serialization
	if raw := string(loaded.RawOptions); !strings.Contains(raw, "postgres") {
		t.Fatalf("expected datasource in raw options, got %q", raw)
	}

	// verify through the JSON API serialization
	jsonRaw, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonRaw), `"datasource"`) {
		t.Fatalf("expected datasource block in collection JSON, got %s", jsonRaw)
	}
}

func TestCollectionDataSourceValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	col := core.NewBaseCollection("test_ds_valid")
	col.DataSource.Type = "oracle"

	if err := app.Save(col); err == nil {
		t.Fatal("expected save to fail with invalid datasource type")
	}
}

func TestCollectionMarshalIncludesBaseOptions(t *testing.T) {
	t.Parallel()

	// ensure we do not break the existing base collection serialization
	// by embedding the (now non-empty) base options struct
	col := core.NewBaseCollection("test_plain")
	raw, err := json.Marshal(col)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	if _, ok := decoded["datasource"]; !ok {
		t.Fatalf("expected a datasource key in the serialized base collection, got %s", raw)
	}

	if _, ok := decoded["type"]; !ok || decoded["type"] != "base" {
		t.Fatalf("expected type base in the serialized collection, got %s", raw)
	}
}
