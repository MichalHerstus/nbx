package nbx

import (
	"slices"

	"github.com/pocketbase/pocketbase/core"
)

// datasourcesList returns a read-only helper listing the available datasource
// types and (optionally) the named credential vault entries.
//
// The response only exposes non-secret configuration info.
type datasourceTypesOutput struct {
	Types   []string `json:"types"`
	Refresh []string `json:"refresh"`
	Secrets []string `json:"secrets"`
}

func datasourcesList(app core.App) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		out := datasourceTypesOutput{
			Types: []string{
				core.DataSourceTypeLocal,
				core.DataSourceTypeMySQL,
				core.DataSourceTypePostgres,
				core.DataSourceTypeMSSQL,
				core.DataSourceTypeREST,
			},
			Refresh: []string{
				core.DataSourceRefreshManual,
				core.DataSourceRefreshCron,
				core.DataSourceRefreshRealtime,
			},
		}

		for name := range app.Settings().Nbx.Secrets {
			out.Secrets = append(out.Secrets, name)
		}
		slices.Sort(out.Secrets)
		if out.Secrets == nil {
			out.Secrets = []string{}
		}

		return e.JSON(200, out)
	}
}
