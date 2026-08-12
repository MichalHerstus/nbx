// Package nbx provides the NextBase HTTP API routes.
//
// The caller (package apis) is responsible for wiring the superuser
// authorization on the provided route group; importing package apis from here
// would create an import cycle.
package nbx

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Register binds the NextBase API routes to the provided (already
// superuser-authorized) route group.
func Register(app core.App, nbxRoute *router.RouterGroup[*core.RequestEvent]) {
	nbxRoute.GET("/datasources", datasourcesList(app))
}
