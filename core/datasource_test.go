package core_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestNewDefaultDataSource(t *testing.T) {
	t.Parallel()

	ds := core.NewDefaultDataSource()

	if ds.Type != core.DataSourceTypeLocal {
		t.Fatalf("expected default type %q, got %q", core.DataSourceTypeLocal, ds.Type)
	}

	if ds.Refresh != core.DataSourceRefreshManual {
		t.Fatalf("expected default refresh %q, got %q", core.DataSourceRefreshManual, ds.Refresh)
	}

	if ds.Method != "GET" {
		t.Fatalf("expected default method GET, got %q", ds.Method)
	}

	if !ds.IsLocal() {
		t.Fatal("expected default datasource to be local")
	}
}

func TestDataSourceTypeChecks(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name        string
		ds          core.DataSource
		expectLocal bool
		expectSQL   bool
		expectREST  bool
	}{
		{"local", core.DataSource{Type: core.DataSourceTypeLocal}, true, false, false},
		{"empty is local", core.DataSource{}, true, false, false},
		{"mysql", core.DataSource{Type: core.DataSourceTypeMySQL}, false, true, false},
		{"postgres", core.DataSource{Type: core.DataSourceTypePostgres}, false, true, false},
		{"mssql", core.DataSource{Type: core.DataSourceTypeMSSQL}, false, true, false},
		{"rest", core.DataSource{Type: core.DataSourceTypeREST}, false, false, true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			if got := s.ds.IsLocal(); got != s.expectLocal {
				t.Fatalf("IsLocal() expected %v, got %v", s.expectLocal, got)
			}
			if got := s.ds.IsExternalSQL(); got != s.expectSQL {
				t.Fatalf("IsExternalSQL() expected %v, got %v", s.expectSQL, got)
			}
			if got := s.ds.IsREST(); got != s.expectREST {
				t.Fatalf("IsREST() expected %v, got %v", s.expectREST, got)
			}
		})
	}
}

func TestDataSourceValidate(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name    string
		ds      core.DataSource
		wantErr bool
	}{
		{"valid local", core.DataSource{Type: core.DataSourceTypeLocal, Refresh: core.DataSourceRefreshManual}, false},
		{"valid rest", core.DataSource{Type: core.DataSourceTypeREST, URL: "https://example.com", Method: "GET"}, false},
		{"invalid type", core.DataSource{Type: "oracle"}, true},
		{"invalid refresh", core.DataSource{Type: core.DataSourceTypeLocal, Refresh: "hourly"}, true},
		{"invalid port", core.DataSource{Type: core.DataSourceTypePostgres, Port: 70000}, true},
		{"invalid url", core.DataSource{Type: core.DataSourceTypeREST, URL: "not-a-url"}, true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := s.ds.Validate()

			if s.wantErr && err == nil {
				t.Fatal("expected validation error, got none")
			}

			if !s.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}

	// ensure that errors are of the expected validation type
	err := core.DataSource{Type: "oracle"}.Validate()
	var validationErrors interface{ Error() string }
	if err == nil || !errors.As(err, &validationErrors) {
		t.Fatalf("expected a validation error type, got %v", err)
	}
}

func TestDataSourceJSONRoundtrip(t *testing.T) {
	t.Parallel()

	ds := core.DataSource{
		Type:          core.DataSourceTypeMySQL,
		CredentialRef: "db_creds",
		Host:          "localhost",
		Port:          3306,
		DB:            "app",
		Table:         "orders",
		Refresh:       core.DataSourceRefreshCron,
	}

	raw, err := json.Marshal(ds)
	if err != nil {
		t.Fatal(err)
	}

	var decoded core.DataSource
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Type != ds.Type || decoded.CredentialRef != ds.CredentialRef ||
		decoded.Host != ds.Host || decoded.Port != ds.Port || decoded.DB != ds.DB ||
		decoded.Table != ds.Table || decoded.Refresh != ds.Refresh {
		t.Fatalf("roundtrip mismatch: %v", decoded)
	}
}
