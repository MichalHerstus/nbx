# NextBase (nbx)

A no-code relational personal-database app (spreadsheet–database hybrid, in the spirit of
Airtable and Tabidoo) built on top of [PocketBase](https://pocketbase.io).

It lets non-technical people build custom data structures, link records, toggle between visual
layouts (grid, kanban, calendar, gallery), create automated workflows, and import/export their
data — inside a single portable Go binary with an embedded SQLite database.

## Feature set (planned)

- **Collections & records** — PocketBase's collection/record model, plus an in-app AI copilot
  that can search/filter data and manage collections in natural language (OpenRouter / local
  Ollama / LM Studio).
- **External datasources** — read-only browsing of tables/views from MySQL, PostgreSQL, MSSQL and
  REST APIs, surfaced as normal collections.
- **Views** — grid, card/kanban, calendar, gallery, and form layouts over any collection.
- **Dashboards & reports** — KPI tiles, data tables, charts, and a map widget (OpenStreetMap via
  Leaflet); printable reports exportable to PDF.
- **Buttons & scripting** — press a button to navigate or run a scripted action.
- **Import / export** — CSV and Excel (via [excelize](https://github.com/qax-os/excelize)), plus
  syncing collections to/from external SQL tables. Available through the web UI and the `nbx` CLI.
- **i18n & theming** — multilingual UI (English and Czech initially, extensible) and user-level
  dark/light + accent-color theming.

## Project status

Under active design/development. See [`project_docs/plan.md`](project_docs/plan.md) for the
approved specification and implementation roadmap.

## Quick start

NextBase is built on the PocketBase source tree (Go 1.25+). No root `main` package exists yet;
the runnable app is produced from `examples/base` and the planned `cmd/nbx` entrypoint:

```sh
go run examples/base/main.go serve --dev
```

## Development

```sh
make test        # go test ./... -v --cover
make lint        # golangci-lint run (v2 config; golangci-lint not bundled)
make jstypes     # regenerate JSVM TypeScript type bindings
```

Frontend is a Vite + shablon SPA in `ui/`: `cd ui && npm run dev`. NextBase-specific UI ships as
a PocketBase UI extension (see `ui-ext/`), not edits to the bundled admin.

## License

[MIT](LICENSE.md). PocketBase and its bundled source keep their original MIT license.
