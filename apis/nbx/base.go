// Package nbx provides the NextBase HTTP API routes.
//
// The caller (package apis / plugins) is responsible for wiring the
// authorization middleware on the provided route groups; importing package
// apis from here would create an import cycle.
package nbx

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Register binds the NextBase superuser-only API routes to the provided
// (already superuser-authorized) route group.
func Register(app core.App, nbxRoute *router.RouterGroup[*core.RequestEvent]) {
	nbxRoute.GET("/datasources", datasourcesList(app))
	nbxRoute.GET("/reports/{id}/pdf", reportsPdf(app))
	nbxRoute.GET("/dashboards/{id}/widgets", dashboardWidgets(app))
}

// RegisterUserApi binds the NextBase user-facing routes (used by the /ui
// frontend) to the provided (already RequireAuth-authorized) route group.
//
// The group is expected to allow any authenticated user - not only
// superusers - so the handlers enforce per-collection rules themselves.
func RegisterUserApi(app core.App, uiGroup *router.RouterGroup[*core.RequestEvent]) {
	uiGroup.GET("/collections", uiCollections(app))
	uiGroup.GET("/view/{id}", uiView(app))
}