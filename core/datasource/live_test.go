package datasource

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// liveTestDSNs holds the connection params for the live integration tests.
// These are the test servers provided for development; the tests skip
// automatically when the servers are unreachable.
var liveTestDSNs = struct {
	MSSQL core.DataSource
	PG    core.DataSource
}{
	MSSQL: core.DataSource{
		Type:          core.DataSourceTypeMSSQL,
		CredentialRef: "mssql_test",
		Host:          "37.46.209.72",
		Port:          1433,
		DB:            "ESPInventura",
		Table:         "Zamestnanec",
	},
	PG: core.DataSource{
		Type:          core.DataSourceTypePostgres,
		CredentialRef: "pg_test",
		Host:          "37.46.209.72",
		Port:          5433,
		DB:            "Test",
		Table:         "sklad_zbozi",
	},
}

func liveCreds(ref, user, pass string) core.Credential {
	return core.Credential{User: user, Password: pass}
}

// requireLive checks that a datasource connection can be opened and pings it,
// skipping the test when the server is unreachable.
func requireLive(t *testing.T, r *Registry, ds core.DataSource, cred core.Credential) {
	t.Helper()

	db, err := r.Get(ds, cred)
	if err != nil {
		t.Skipf("skip: unable to open %s connection: %v", ds.Type, err)
	}
	if err := db.DB().Ping(); err != nil {
		t.Skipf("skip: unable to ping %s: %v", ds.Type, err)
	}
}

func TestLiveSQLList(t *testing.T) {
	mssqlCol := liveCollection("mssql_zamestnanec", liveTestDSNs.MSSQL, []string{"CeleJmeno", "Poznamka"})
	pgCol := liveCollection("pg_sklad_zbozi", liveTestDSNs.PG, []string{"pn", "pn_nazev", "segment"})

	scenarios := []struct {
		name   string
		col    *core.Collection
		cred   core.Credential
		expect int
	}{
		{
			"mssql",
			mssqlCol,
			liveCreds("mssql_test", "sa", "qS5xIVbodUJH"),
			17,
		},
		{
			"postgres",
			pgCol,
			liveCreds("pg_test", "postgres", "hnLKPoUPu3k"),
			17,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			r := NewRegistry()
			defer r.Close()

			requireLive(t, r, s.col.GetDataSource(), s.cred)

			result, err := r.List(s.col, s.cred, 1, 10, "", "")
			if err != nil {
				t.Fatalf("list failed: %v", err)
			}

			if result.TotalItems != s.expect {
				t.Fatalf("expected %d total items, got %d", s.expect, result.TotalItems)
			}

			if len(result.Items.([]*core.Record)) != 10 {
				t.Fatalf("expected 10 items on page 1, got %d", len(result.Items.([]*core.Record)))
			}
		})
	}
}

func TestLiveSQLSortAndFilter(t *testing.T) {
	col := liveCollection("pg_sklad_zbozi", liveTestDSNs.PG, []string{"pn", "pn_nazev"})
	cred := liveCreds("pg_test", "postgres", "hnLKPoUPu3k")

	r := NewRegistry()
	defer r.Close()

	requireLive(t, r, col.GetDataSource(), cred)

	// case-insensitive contains filter + descending sort by pn
	// ("Manipulace" segment has 2 rows: PN000006, PN000007)
	filtered, err := r.List(col, cred, 1, 20, "-pn", "segment = 'Manipulace'")
	if err != nil {
		t.Fatalf("filtered/sorted list failed: %v", err)
	}

	if filtered.TotalItems != 2 {
		t.Fatalf("expected 2 filtered rows, got %d", filtered.TotalItems)
	}

	items := filtered.Items.([]*core.Record)

	// verify it is sorted descending by pn
	first := items[0]
	prev := first.GetString("pn")
	for _, rec := range items[1:] {
		cur := rec.GetString("pn")
		if prev < cur {
			t.Fatalf("expected descending pn order, got %q then %q", prev, cur)
		}
		prev = cur
	}
}

func TestLiveSQLOnlyReads(t *testing.T) {
	// external records should not have a PK in the PB store; listing only.
	col := liveCollection("mssql_zamestnanec", liveTestDSNs.MSSQL, []string{"CeleJmeno"})
	cred := liveCreds("mssql_test", "sa", "qS5xIVbodUJH")

	r := NewRegistry()
	defer r.Close()

	requireLive(t, r, col.GetDataSource(), cred)

	result, err := r.List(col, cred, 2, 5, "", "")
	if err != nil {
		t.Fatalf("page 2 list failed: %v", err)
	}

	if result.Page != 2 || len(result.Items.([]*core.Record)) != 5 {
		t.Fatalf("expected page 2 with 5 items, got page %d with %d items", result.Page, len(result.Items.([]*core.Record)))
	}

	// pagination math: 17 total / 5 perPage = 4 pages
	if result.TotalPages != 4 {
		t.Fatalf("expected 4 total pages, got %d", result.TotalPages)
	}
}

// liveCollection builds a base collection whose fields map to external
// columns (the primary key field is mapped to the external id/ID column).
func liveCollection(name string, ds core.DataSource, fields []string) *core.Collection {
	col := core.NewBaseCollection(name)
	col.DataSource = ds

	// primary key text field backed by the external id/ID column
	col.Fields.Add(&core.TextField{Name: "id", PrimaryKey: true})

	for _, f := range fields {
		col.Fields.Add(&core.TextField{Name: f})
	}

	return col
}
