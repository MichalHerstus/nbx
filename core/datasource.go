package core

import (
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/ozzo-validation/v4/is"
)

// NextBase meta collection names (P0).
const (
	CollectionNameDatasources = "_datasources"
	CollectionNameViews       = "_views"
	CollectionNameDashboards  = "_dashboards"
	CollectionNameReports     = "_reports"
	CollectionNameButtons     = "_buttons"
	CollectionNamePreferences = "_preferences"
)

const (
	DataSourceTypeLocal    = "local"
	DataSourceTypeMySQL    = "mysql"
	DataSourceTypePostgres = "postgres"
	DataSourceTypeMSSQL    = "mssql"
	DataSourceTypeREST     = "rest"
)

const (
	DataSourceRefreshManual   = "manual"
	DataSourceRefreshCron     = "cron"
	DataSourceRefreshRealtime = "realtime"
)

// DataSource defines the external datasource configuration of a collection.
//
// It is stored (by reference) as part of the collection "base" options and
// its non-secret fields are exposed through the collections API. Secret values
// (credentials) are never stored inline - they are referenced by name via
// [DataSource.CredentialRef] and resolved from the Settings Nbx vault at runtime.
type DataSource struct {
	// Type specifies the datasource type:
	// "local" (default), "mysql", "postgres", "mssql" or "rest".
	Type string `form:"type" json:"type"`

	// CredentialRef is the name of the credential entry in the Settings
	// Nbx.Secrets vault that holds the secret values (user/password/apiKey/token).
	CredentialRef string `form:"credentialRef" json:"credentialRef"`

	// SQL dialect related (non-secret) fields.
	Host  string `form:"host" json:"host"`
	Port  int    `form:"port" json:"port"`
	DB    string `form:"db" json:"db"`
	SSL   bool   `form:"ssl" json:"ssl"`
	Table string `form:"table" json:"table"`
	Query string `form:"query" json:"query"`

	// REST related (non-secret) fields.
	URL      string            `form:"url" json:"url"`
	Method   string            `form:"method" json:"method"`
	Headers  map[string]string `form:"headers" json:"headers"`
	JSONPath string            `form:"jsonPath" json:"jsonPath"`
	Auth     string            `form:"auth" json:"auth"`

	// Refresh controls how the datasource data is refreshed:
	// "manual" (default), "cron" or "realtime".
	Refresh string `form:"refresh" json:"refresh"`
}

// NewDefaultDataSource returns a DataSource with the default local configuration.
func NewDefaultDataSource() DataSource {
	return DataSource{
		Type:    DataSourceTypeLocal,
		Refresh: DataSourceRefreshManual,
		Method:  "GET",
	}
}

// IsLocal checks whether the datasource points to the local SQLite database.
func (d DataSource) IsLocal() bool {
	return d.Type == "" || d.Type == DataSourceTypeLocal
}

// IsExternalSQL checks whether the datasource points to an external SQL database.
func (d DataSource) IsExternalSQL() bool {
	return d.Type == DataSourceTypeMySQL ||
		d.Type == DataSourceTypePostgres ||
		d.Type == DataSourceTypeMSSQL
}

// IsREST checks whether the datasource points to a REST endpoint.
func (d DataSource) IsREST() bool {
	return d.Type == DataSourceTypeREST
}

// Validate makes DataSource validatable by implementing the [validation.Validatable] interface.
func (d DataSource) Validate() error {
	return validation.ValidateStruct(&d,
		validation.Field(
			&d.Type,
			validation.In(DataSourceTypeLocal, DataSourceTypeMySQL, DataSourceTypePostgres, DataSourceTypeMSSQL, DataSourceTypeREST),
		),
		validation.Field(&d.CredentialRef, validation.Length(0, 255)),
		validation.Field(&d.Host, is.Host),
		validation.Field(&d.Port, validation.Min(0), validation.Max(65535)),
		validation.Field(&d.URL, is.URL),
		validation.Field(
			&d.Refresh,
			validation.In(DataSourceRefreshManual, DataSourceRefreshCron, DataSourceRefreshRealtime),
		),
	)
}
