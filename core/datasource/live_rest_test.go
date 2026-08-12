package datasource

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// restfulAPIDevURL is a free public REST service used for live REST testing.
// The test skips automatically when the service is unreachable.
const restfulAPIDevURL = "https://api.restful-api.dev/objects"

func TestRESTLive(t *testing.T) {
	col := testRestCollection(restfulAPIDevURL, "")

	r := NewRegistry()
	defer r.Close()

	result, err := r.List(col, core.Credential{}, 1, 5, "name", "")
	if err != nil {
		t.Skipf("skip: restful-api.dev unreachable: %v", err)
	}

	if result.TotalItems < 5 {
		t.Fatalf("expected at least 5 total items, got %d", result.TotalItems)
	}

	items := result.Items.([]*core.Record)
	if len(items) != 5 {
		t.Fatalf("expected 5 items on page 1, got %d", len(items))
	}

	// ensure sorting by name is applied (ascending)
	prev := items[0].GetString("name")
	for _, rec := range items[1:] {
		cur := rec.GetString("name")
		if cur < prev {
			t.Fatalf("expected ascending name order, got %q then %q", cur, prev)
		}
	}
}
