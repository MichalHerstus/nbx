package datasource

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// testRestCollection builds a collection backed by a REST datasource with the
// given URL.
func testRestCollection(url, jsonPath string) *core.Collection {
	col := core.NewBaseCollection("rest_api")
	col.DataSource = core.DataSource{
		Type:     core.DataSourceTypeREST,
		URL:      url,
		Method:   http.MethodGet,
		JSONPath: jsonPath,
		Refresh:  core.DataSourceRefreshManual,
	}
	col.Fields.Add(&core.TextField{Name: "id", PrimaryKey: true})
	col.Fields.Add(&core.TextField{Name: "name"})
	col.Fields.Add(&core.TextField{Name: "category"})

	return col
}

func TestRESTList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"3","name":"Charlie","category":"B"},
			{"id":"1","name":"Alice","category":"A"},
			{"id":"2","name":"Bob","category":"A"}
		]`))
	}))
	defer server.Close()

	col := testRestCollection(server.URL, "")

	r := NewRegistry()
	defer r.Close()

	result, err := r.List(col, core.Credential{}, 1, 10, "name", "")
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalItems != 3 {
		t.Fatalf("expected 3 total items, got %d", result.TotalItems)
	}

	items := result.Items.([]*core.Record)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// sorted ascending by name -> Alice, Bob, Charlie
	if items[0].GetString("name") != "Alice" {
		t.Fatalf("expected first item Alice, got %q", items[0].GetString("name"))
	}
	if items[2].GetString("name") != "Charlie" {
		t.Fatalf("expected last item Charlie, got %q", items[2].GetString("name"))
	}
}

func TestRESTFilterAndPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"id":"1","name":"Alice","category":"A"},
			{"id":"2","name":"Bob","category":"A"},
			{"id":"3","name":"Charlie","category":"B"}
		]`))
	}))
	defer server.Close()

	col := testRestCollection(server.URL, "data.items")
	// override to no jsonPath for the flat test
	col.DataSource.JSONPath = ""

	r := NewRegistry()
	defer r.Close()

	// filter category A -> 2 items, page 1 perPage 1 -> 1 item, 2 pages
	result, err := r.List(col, core.Credential{}, 1, 1, "", "category = 'A'")
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalItems != 2 {
		t.Fatalf("expected 2 filtered total items, got %d", result.TotalItems)
	}
	if result.TotalPages != 2 {
		t.Fatalf("expected 2 total pages, got %d", result.TotalPages)
	}
	if len(result.Items.([]*core.Record)) != 1 {
		t.Fatalf("expected 1 item on page 1, got %d", len(result.Items.([]*core.Record)))
	}
}

func TestRESTJSONPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[{"id":"9","name":"Zed","category":"Z"}]}}`))
	}))
	defer server.Close()

	col := testRestCollection(server.URL, "data.items")

	r := NewRegistry()
	defer r.Close()

	result, err := r.List(col, core.Credential{}, 1, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	items := result.Items.([]*core.Record)
	if len(items) != 1 || items[0].GetString("name") != "Zed" {
		t.Fatalf("unexpected jsonPath result: %+v", result.Items)
	}
}

func TestRESTInjectionBlocked(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"1","name":"A","category":"X"}]`))
	}))
	defer server.Close()

	col := testRestCollection(server.URL, "")

	r := NewRegistry()
	defer r.Close()

	// only simple equality is supported for REST filters
	_, err := r.List(col, core.Credential{}, 1, 10, "", "name ~ 'A'")
	if err == nil {
		t.Fatal("expected an error for a non-equality REST filter")
	}
}

func TestRegistryCacheAndReap(t *testing.T) {
	t.Parallel()

	ds := core.DataSource{
		Type: core.DataSourceTypeMySQL,
		Host: "10.0.0.1",
		Port: 3306,
		DB:   "test",
	}

	r := NewRegistry()

	// opening should not error (dsn is not validated at open time)
	db1, err := r.Get(ds, core.Credential{User: "u", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}
	db2, err := r.Get(ds, core.Credential{User: "u", Password: "p"})
	if err != nil {
		t.Fatal(err)
	}

	// same (driver+dsn) -> same cached connection
	if db1 != db2 {
		t.Fatal("expected the cached connection to be reused")
	}

	if r.Len() != 1 {
		t.Fatalf("expected 1 cached connection, got %d", r.Len())
	}

	// different password -> different connection
	db3, err := r.Get(ds, core.Credential{User: "u", Password: "other"})
	if err != nil {
		t.Fatal(err)
	}
	_ = db3
	if r.Len() != 2 {
		t.Fatalf("expected 2 cached connections, got %d", r.Len())
	}

	r.Reap()
	_ = db1

	r.Close()
	if r.Len() != 0 {
		t.Fatalf("expected registry to be empty after Close, got %d", r.Len())
	}
}
