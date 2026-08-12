package datasource

import (
	"fmt"
	"net/url"

	"github.com/pocketbase/pocketbase/core"

	// register the external SQL drivers for the supported dialects
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
)

// External SQL driver names used with database/sql.
const (
	driverMySQL    = "mysql"
	driverPostgres = "pgx"
	driverMSSQL    = "sqlserver"
)

// driverFor returns the database/sql driver name for the provided
// datasource type. It returns an empty string for non-SQL types.
func driverFor(dsType string) string {
	switch dsType {
	case core.DataSourceTypeMySQL:
		return driverMySQL
	case core.DataSourceTypePostgres:
		return driverPostgres
	case core.DataSourceTypeMSSQL:
		return driverMSSQL
	}

	return ""
}

// defaultPort returns the default port for the provided SQL dialect.
func defaultPort(dsType string) int {
	switch dsType {
	case core.DataSourceTypeMySQL:
		return 3306
	case core.DataSourceTypePostgres:
		return 5432
	case core.DataSourceTypeMSSQL:
		return 1433
	}

	return 0
}

// builderNameFor returns the dbx builder key for the provided SQL type.
//
// It may differ from the database/sql driver name (eg. go-mssqldb registers
// "sqlserver" but dbx keys its MSSQL builder as "mssql").
func builderNameFor(dsType string) string {
	switch dsType {
	case core.DataSourceTypeMySQL:
		return driverMySQL
	case core.DataSourceTypePostgres:
		return driverPostgres
	case core.DataSourceTypeMSSQL:
		return "mssql"
	}

	return ""
}

// buildDSN builds a driver specific DSN string from the provided
// non-secret datasource config and the resolved credential.
//
// Only external SQL (mysql/postgres/mssql) types are supported.
func buildDSN(ds core.DataSource, cred core.Credential) (driver, dsn string, err error) {
	driver = driverFor(ds.Type)
	if driver == "" {
		return "", "", fmt.Errorf("unsupported SQL datasource type %q", ds.Type)
	}

	port := ds.Port
	if port <= 0 {
		port = defaultPort(ds.Type)
	}

	switch ds.Type {
	case core.DataSourceTypeMySQL:
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?parseTime=true",
			cred.User,
			cred.Password,
			ds.Host,
			port,
			ds.DB,
		)
	case core.DataSourceTypePostgres:
		sslMode := "disable"
		if ds.SSL {
			sslMode = "require"
		}

		pw := url.QueryEscape(cred.Password)
		dsn = fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			url.QueryEscape(cred.User),
			pw,
			ds.Host,
			port,
			ds.DB,
			sslMode,
		)
	case core.DataSourceTypeMSSQL:
		u := &url.URL{
			Scheme: "sqlserver",
			User:   url.UserPassword(cred.User, cred.Password),
			Host:   fmt.Sprintf("%s:%d", ds.Host, port),
		}
		q := u.Query()
		q.Set("database", ds.DB)
		if ds.SSL {
			q.Set("encrypt", "true")
		} else {
			q.Set("encrypt", "disable")
		}
		u.RawQuery = q.Encode()

		dsn = u.String()
	default:
		return "", "", fmt.Errorf("unsupported SQL datasource type %q", ds.Type)
	}

	return
}
