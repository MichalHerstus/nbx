package datasource

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestBuildDSN(t *testing.T) {
	t.Parallel()

	ds := core.DataSource{
		Host: "db.example.com",
		DB:   "mydb",
	}
	cred := core.Credential{User: "admin", Password: "secret"}

	scenarios := []struct {
		name   string
		typ    string
		port   int
		ssl    bool
		expect []string // substrings expected in the DSN
	}{
		{
			"mysql",
			core.DataSourceTypeMySQL,
			3306,
			false,
			[]string{"admin:secret@tcp(db.example.com:3306)/mydb?parseTime=true"},
		},
		{
			"postgres default port",
			core.DataSourceTypePostgres,
			0,
			false,
			[]string{"postgres://", "@db.example.com:5432/mydb?sslmode=disable"},
		},
		{
			"postgres ssl",
			core.DataSourceTypePostgres,
			5433,
			true,
			[]string{"@db.example.com:5433/mydb?sslmode=require"},
		},
		{
			"mssql",
			core.DataSourceTypeMSSQL,
			1433,
			false,
			[]string{"sqlserver://", "@db.example.com:1433?database=mydb"},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			ds.Type = s.typ
			ds.Port = s.port
			ds.SSL = s.ssl

			driver, dsn, err := buildDSN(ds, cred)
			if err != nil {
				t.Fatal(err)
			}

			if driver == "" {
				t.Fatal("expected a non-empty driver")
			}

			for _, e := range s.expect {
				if !strings.Contains(dsn, e) {
					t.Fatalf("expected DSN to contain %q, got %q", e, dsn)
				}
			}

			// ensure no secret leaks into the DSN for logging safety is not the
			// concern here, but the driver must be registered
			if driver != driverFor(s.typ) {
				t.Fatalf("unexpected driver %q", driver)
			}
		})
	}
}

func TestBuildDSNUnsupported(t *testing.T) {
	t.Parallel()

	_, _, err := buildDSN(core.DataSource{Type: core.DataSourceTypeREST}, core.Credential{})
	if err == nil {
		t.Fatal("expected an error for an unsupported (rest) build")
	}
}

func TestDriverFor(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		typ       string
		expect    string
		expectErr bool
	}{
		{core.DataSourceTypeMySQL, "mysql", false},
		{core.DataSourceTypePostgres, "pgx", false},
		{core.DataSourceTypeMSSQL, "sqlserver", false},
		{core.DataSourceTypeREST, "", true},
		{core.DataSourceTypeLocal, "", true},
	}

	for _, s := range scenarios {
		got := driverFor(s.typ)
		if s.expectErr && got != "" {
			t.Fatalf("expected empty driver for %q", s.typ)
		}
		if !s.expectErr && got != s.expect {
			t.Fatalf("expected driver %q for %q, got %q", s.expect, s.typ, got)
		}
	}
}
