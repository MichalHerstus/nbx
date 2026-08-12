# AGENTS.md

## What this repo is
- A **checkout of the PocketBase v0.39 source tree** (Go library, module `github.com/pocketbase/pocketbase`), used as the base for the **NextBase (nbx)** personal-database app. You edit PB internals directly (`core/`, `apis/`, `plugins/`, `forms/`, `tools/`, `ui/`), you do not consume it as a dependency.
- No `.git` directory — do not attempt git operations.

## Read first
- `project_docs/plan.md` — the approved NextBase spec (datasources, views, dashboards/reports/PDF, buttons/scripting, import/export via `qax-os/excelize`, AI assistant, i18n EN/CZ, theming). Feature/phase work must follow it.
- `.opencode/skills/pocketbase/SKILL.md` and `.opencode/skills/pocketbase-api-add-field/SKILL.md` — auto-loaded PB skills; consult for schema/API work.
- `pocketbase.go` / `examples/base/main.go` — app bootstrap pattern (`pocketbase.New()` + `app.Start()`).

## Commands (Go 1.25+; go 1.26.3 installed)
- `make test` → `go test ./... -v --cover`; single package/test: `go test ./core -run TestName -v`
- `make lint` → `golangci-lint run -c ./golangci.yml ./...` (v2 config; `golangci-lint` is **not installed** in this environment)
- `make jstypes` → regenerates JSVM TS types (`go run ./plugins/jsvm/internal/types/types.go`); run after changing JSVM bindings
- `make test-report`
- Root package has **no `main.go`** — `go run .` fails. Runnable app is built/run from `examples/base` (`go run examples/base/main.go serve --dev`) or a future `cmd/nbx`.

## Frontend (pb-admin SPA in `ui/`)
- Vanilla **Vite + shablon** SPA (`t`, `store`, `watch` from `<script>` globals; no framework). `leaflet` and `uplot` are already bundled as static libs in `ui/public/libs/`.
- Embedded into the Go binary via `ui/embed.go` (`//go:embed all:dist`). `ui/dist` is committed; `ui/node_modules` is **not** — type changes require `cd ui && npm install` first.
- Dev loop: `cd ui && npm run dev` (vite) or `npm run build` (= `dprint fmt && vite build`, then re-run Go so the new dist is embedded).
- NextBase frontend must ship as a **UI extension**, not edits to bundled `ui/src`: register a `core.UIExtension` (served at `/_/extensions.js`, see `apis/extensions.go`, populated via `ServeEvent.UIExtensions` in `core/events.go`) — e.g. `ui-ext/nbx/`.

## Backend gotchas when editing `core/`
- Hooks live in `tools/hook` and are registered on `*BaseApp` (`core/base.go`) as `app.On<Subject>...`. Tagged hooks take collection names as variadic args, e.g. `OnRecordCreate("articles")`. Naming flow: `…Validate → …Create → …CreateExecute → …AfterCreateSuccess/…AfterCreateError`.
- `DB()` returns a `dualDBBuilder`: SELECTs route to a concurrent builder, writes/DDL to a non-concurrent one; inside `RunInTransaction` both collapse to the tx builder. Collection schema JSON + per-collection tables are synced by `SyncRecordTableSchema`.
- Core is **SQLite-only** (`modernc.org/sqlite`, no CGO) and saturated with SQLite-isms: `PRAGMA_TABLE_INFO`, `sqlite_master`/`schemas`, WAL/`rowid`, JSON column types, `SaveView`. For external SQL sources, route around these at `apis/record_crud.go` (see plan F1) — do not reuse `SaveView`.
- Per-collection settings (views/dashboards/buttons/prefs) are stored as PB **meta collections** (`_views`, etc.), not Go files.

## Conventions
- No code comments unless required; match existing style (tabs, `db:"…"` tags, `FieldName`-style consts).
- Don't add heavy deps casually — plan names the approved exceptions (`excelize`, PDF renderer, AI). Confirm against `go.mod` first.