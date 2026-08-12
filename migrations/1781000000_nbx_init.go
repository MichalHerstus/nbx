package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		collections := []*core.Collection{}

		datasources := core.NewBaseCollection(core.CollectionNameDatasources)
		datasources.System = true
		datasources.Fields.Add(&core.TextField{Name: "label"})
		datasources.Fields.Add(&core.SelectField{
			Name:      "type",
			MaxSelect: 1,
			Values:    []string{"local", "mysql", "postgres", "mssql", "rest"},
		})
		datasources.Fields.Add(&core.JSONField{Name: "config"})
		collections = append(collections, datasources)

		views := core.NewBaseCollection(core.CollectionNameViews)
		views.System = true
		views.Fields.Add(&core.TextField{Name: "label"})
		views.Fields.Add(&core.TextField{Name: "sourceCollection"})
		views.Fields.Add(&core.SelectField{
			Name:      "type",
			MaxSelect: 1,
			Values:    []string{"grid", "kanban", "calendar", "gallery", "form"},
		})
		views.Fields.Add(&core.JSONField{Name: "config"})
		collections = append(collections, views)

		dashboards := core.NewBaseCollection(core.CollectionNameDashboards)
		dashboards.System = true
		dashboards.Fields.Add(&core.TextField{Name: "label"})
		dashboards.Fields.Add(&core.JSONField{Name: "config"})
		collections = append(collections, dashboards)

		reports := core.NewBaseCollection(core.CollectionNameReports)
		reports.System = true
		reports.Fields.Add(&core.TextField{Name: "label"})
		reports.Fields.Add(&core.TextField{Name: "dashboard"})
		reports.Fields.Add(&core.JSONField{Name: "config"})
		collections = append(collections, reports)

		buttons := core.NewBaseCollection(core.CollectionNameButtons)
		buttons.System = true
		buttons.Fields.Add(&core.TextField{Name: "label"})
		buttons.Fields.Add(&core.SelectField{
			Name:      "action",
			MaxSelect: 1,
			Values:    []string{"open_page", "run_js", "webhook"},
		})
		buttons.Fields.Add(&core.TextField{Name: "target"})
		buttons.Fields.Add(&core.JSONField{Name: "config"})
		collections = append(collections, buttons)

		preferences := core.NewBaseCollection(core.CollectionNamePreferences)
		preferences.System = true
		preferences.Fields.Add(&core.TextField{Name: "userKey"})
		preferences.Fields.Add(&core.JSONField{Name: "config"})
		collections = append(collections, preferences)

		for _, col := range collections {
			if err := txApp.Save(col); err != nil {
				return err
			}
		}

		return nil
	}, func(txApp core.App) error {
		tables := []string{
			core.CollectionNameDatasources,
			core.CollectionNameViews,
			core.CollectionNameDashboards,
			core.CollectionNameReports,
			core.CollectionNameButtons,
			core.CollectionNamePreferences,
		}

		for _, name := range tables {
			if _, err := txApp.DB().DropTable(name).Execute(); err != nil {
				return err
			}
		}

		return nil
	})
}
