# NextBase (nbx) — Specification & Implementation Plan

Application name: **NextBase**
Shortcut / binary name: **nbx**
Status: **Approved plan (not yet implemented)**
Target repo: PocketBase fork (Go, v0.39.x in this repo)
Date: 2026-08-12

## 0. What we're building
An Airtable/Tabidoo-style personal database layer **inside PocketBase** — extending its
collection/record model with external datasources, multiple visual views, dashboards/reports,
button scripting, import/export, an in-app AI assistant, multilingual UI (EN/CZ), and user-level
theming.

Core principle: **reuse PB's existing machinery** (collections, records API, view collections,
hooks/JSVM, SPA + shablon, UI extension mechanism, multi-dialect `dbx`) rather than importing
heavy new modules where possible.

## 1. Architecture fit — what PB already gives us (verified in code)

| Need | Existing PB asset | Seam |
|---|---|---|
| Data model | `_collections` JSON row + per-collection SQLite table; `view` collections with `viewQuery` | `core/collection_model_*.go`, `core/view.go` |
| CRUD API | `/api/collections/{c}/records` | `apis/record_crud.go` |
| Declarative field rendering | `app.fieldTypes.*` registry (view/input/settings) | `ui/src/fields/*` |
| SPA pages/routes | hash router + `app.routes.*`, `app.components.*` | `ui/src/router.js`, `ui/src/main.js` |
| UI extension (no fork of bundled UI) | `UIExtension` -> `/_/extensions.js` | `apis/extensions.go`, `core/events.go` |
| Scripting/automation | JSVM: `pb_hooks/*.pb.js` -> `on*`, `$app`, `$dbx`, `$http`, `routerAdd`, `cronAdd` | `plugins/jsvm/` |
| Multi-dialect SQL builder | `dbx` already supports sqlite/mysql/postgres/mssql dialects | `pocketbase/dbx` builder_*.go |
| Import/export | **none** — CSV/Excel must be added | new |

### Key architectural decisions
1. **External SQL != reusable via `SaveView`.** PB creates physical SQLite VIEWs from
   `viewQuery`; SQLite views can't span remote MySQL/Postgres. External tables need a **new
   execution path**, not the view mechanism. -> Introduce a `DataSource` abstraction; the records
   API routes by `collection.source.type`.
2. **Views and dashboards are separate from Sources.** A "source" is where data lives (local
   collection / external SQL / REST). A "view" is *how it's displayed* (grid/kanban/calendar/
   gallery + fields/filter/sort/grouping). Declarative JSON config -> SPA renders everything from
   config with no per-layout Go code.
3. **Config stored as PB collections** (self-describing; gets CRUD, permissions, realtime for
   free): `_datasources`, `_views`, `_dashboards`, `_reports`, `_buttons`, `_preferences`.
4. **All new UI ships as a UI extension** (`/_/extensions/nbx/...`) — zero changes to bundled
   `ui/src`.

## 2. Feature specifications

### F1 — External datasources (READ-ONLY)
Decision: external SQL/REST sources are **read-only** — browse/search/sort/export; editing stays
in PB-local collections. No cross-dialect DDL/schema sync.

New `source` block on every collection:
```
datasource: {
  type: "local" | "mysql" | "postgres" | "mssql" | "rest",
  dsn / host,port,user,password,db, ssl,        // SQL dialects
  table / query,
  url, method, headers, jsonPath, auth,          // REST
  refresh: "manual" | "cron" | "realtime"
}
```
Backend: `DataSourceRegistry` with a pool of `*sql.DB` per dialect wrapped as dialect-aware
`dbx.DB` (dbx already has builders). REST fetch + TTL cache. Records API (`record_crud.go`)
branches on `collection.datasource.type`; non-local -> external executor for `list/view`
(read-only). Read-only enforcement + `LastSyncAt`/`Error` surface in UI.

### F2 — Views (list/grid, card/kanban, calendar, gallery, form)
`_views` collection storing declarative config:
```
{ sourceCollection, type: grid|kanban|calendar|gallery|form,
  fields, sort, filter (search.FilterData), groupBy, layout/split,
  title, primaryField }
```
Backend mostly unnecessary — views read via existing `/records` list API (`filter`, `sort`,
`groupBy`). Frontend: `app.components.datasView` renders per type, **reuses
`app.fieldTypes[type].view`** for every cell; form reuses `recordUpsertModal.js` input registry.
Kanban groups by `select/relation`; calendar by `date`; gallery thumbnails by `file`.

### F3 — Dashboards, reports, PDF (server-side HTML->PDF)
`_dashboards`: layout grid of widgets — KPI tile (count/sum/avg/min/max), data table (reuse
grid), chart (reuse bundled `uplot`), text/note, **map** (OSM via bundled Leaflet). Each widget
= `{ source, aggregation|filter, type, metrics }`. `_reports` = a dashboard + print stylesheet.
Decision: **server-side HTML->PDF renderer** via a small, swappable `PdfRenderer` interface.
Endpoint `GET /api/nbx/reports/{id}/pdf` renders widget tables/aggregations server-side into
HTML (reuse mails templating) -> renderer -> served as `.pdf`.

**Map widget (v1):**
- Display-only watchdog rendering a **multi-marker** read-only Leaflet map of records from a
  `source`, using the bundled OSM tile layer (`tile.openstreetmap.org`) + marker icons already in
  `ui/src/base/leaflet.js` — **no new dependency**, reuses PB's `geoPoint` data field.
- Config: `{ type: "map", source, geoField (geoPoint), titleField?, filter, cluster: bool, zoom, center }`.
- New component `app.components.nbxMap` (in `ui-ext/nbx/`): fetches via existing `/records` list
  API (`filter`/`sort`), one marker per record, `titleField` as popup; optional marker clustering
  for many points.
- Backend: no new Go route (reads through the standard list API, like grid/table widgets).
- **PDF consideration:** interactive Leaflet can't render server-side. v1 renders coordinates/
  titled locations as a small table in the printed report; a static OSM map-image overlay is a
  follow-up option.

### F4 — Buttons & simple scripting
`_buttons`: `{ label, action: open_page | run_js | webhook, target, args }`.
- `open_page`: first-class, handled purely in UI (no server) — "press 'Goto Orders' -> open page Orders".
- `run_js`: `POST /api/nbx/buttons/{id}/run` executes a JS snippet through the existing JSVM
  (`$app`, `$dbx`, `$http`) with a safe whitelist.
- Advanced automation reuses PB's own surface (`cronAdd`, `on*` hooks).

### F5 — Import / Export (CSV, Excel) + SQL table targets
**File targets (HTTP):**
- Export `GET /api/nbx/export/{collection}.csv|xlsx` — stream via existing list query; CSV via
  stdlib `encoding/csv`; XLSX via **excelize** streaming writer (`StreamWriter`).
- Import `POST /api/nbx/import/{collection}` — parse via **excelize** (`StreamReader`) / CSV ->
  map columns to fields -> batch create through existing RecordCreate path (validations + hooks
  apply for free).
- Dependency: `github.com/qax-os/excelize` (standard Go XLSX lib; also covers `.xlsm`/CSV and
  cell styling for reports).

**SQL table targets (HTTP + CLI)** — one-shot data movement between a PB collection and an
external SQL table (**SQL only**: MySQL/Postgres/MSSQL; REST excluded). Reuses the F1 connection
registry (`core/datasource`) and the shared IO engine (`svcs/io`) — the same engine serves both
the HTTP (`/api/nbx/export/db`, `/api/nbx/import/db`) and the CLI.
- **Export -> external table:** push records (optional `filter`/`sort`) into an external SQL
  table; **auto-create** the table if missing via dialect DDL from the collection schema
  (field->column type mapping); default **append**, optional `--replace` (truncate first).
- **Import <- external table:** `SELECT` rows -> map columns to collection fields by name ->
  batch-create through the standard RecordCreate path (validations + hooks apply). Import into an
  **existing** collection, or `--create-collection <name>` to infer fields from the table columns
  and create a new PB collection (best-effort types — review before use).
- **Type mapping (`svcs/io`):** text/number/bool/date map natively per dialect; `relation`/
  `select`/`json`/`geoPoint`/`file` JSON-encoded into a single column.
- **Connections:** named datasource (`_datasources`, DSNs stored once) or inline
  `--driver/--dsn/--table` in the CLI; HTTP/UI use saved datasources by id.
- **CLI (separate subcommands):**
  ```
  ./nbx export-db <collection> --datasource <name|id> | --driver <mysql|postgres|mssql> --dsn "..." [--table T] [--replace] [--filter "..."] [--sort "..."]
  ./nbx import-db <collection> --datasource <name|id> | --driver ... --dsn "..." [--table T] [--replace] [--limit N] [--create-collection <name>]
  ```
- New: `cmd/nbx/export-db.go`, `cmd/nbx/import-db.go`; `svcs/io/dbtable.go` (+ per-dialect DDL +
  column mapping). Cobra commands run against the app instance directly (full access to `pb_data`,
  no auth token, works without a running server).

### F6 — AI assistant (workspace copilot)
- **Provider-agnostic OpenAI-compatible client** (`core/ai` `LLMClient`); config selects
  `openrouter` (cloud) | `ollama`/`lmstudio` (local, `localhost:11434`/`:1234`). One code path,
  no vendor SDKs.
- **Tool loop over a PB tool registry** — each tool wraps an existing PB operation with an OpenAI
  function schema:
  - Read (auto-run): `listCollections`, `searchRecords(filter)`, `getRecord`, `listViews`, aggregations
  - Write (proposed -> user-approved): `createCollection`, `addField`, `updateCollection`, `createView`, `createDashboard`, `createButton`
- **Safety model:** read-only queries execute automatically; every schema mutation is rendered as
  a confirmable action card (reusing existing forms/modals + `pb.js` mutation events) before
  executing through the standard APIs, so PB hooks/validation still apply.
- **Persistent copilot side-panel:** docked chat across collections/views/dashboards,
  context-aware of the currently open source/view; SSE streaming for token-by-token responses.
- Backend: `core/ai/` + `apis/ai/chat.go` (`POST /api/ai/chat`, superuser-only, SSE) + provider
  config page. Frontend: UI-extension chat panel (shablon), reusing store/watch, modal, field-type
  rendering.

### F7 — Multilanguage UI (EN + CZ initially, extensible)
PB's bundled SPA has **no i18n** (hardcoded English), so our extension carries its own layer.
- i18n module in the UI extension: dictionaries (`en`, `cz`) + reactive `$t(key)` helper + a
  locale store; runtime switch with in-place re-render (no reload).
- Extensible: adding a language = drop in a dictionary + register it (designed for the later
  "more languages" goal).
- Locale on **user level** (persisted per-user in the prefs collection), default EN, fallback EN
  with visible missing-key warnings in dev.
- **Scope: UI strings only** (menus, labels, buttons, tooltips in the extension). Bundled PB
  admin strings are out of scope (would require forking); user-authored content stays as typed.

### F8 — Dark/Light theme + user-level theme colors
PB already ships light/dark + a **global** accent (`app.store.activeColorScheme`,
`vars.css` CSS vars, `settings.meta.accentColor` via store.js:272). New capability:
**per-user** theme.
- Light/dark: reuse existing `app.store.activeColorScheme` + `vars.css` (no new mechanism).
  Extension components use the same CSS vars -> inherit automatically.
- User-level theme colors: per-user theme object (accent color + optional explicit light/dark
  override) persisted in the prefs collection, applied via
  `document.documentElement.style.setProperty("--accentColor", ...)` (same pattern as store.js:272)
  so all components (ours and PB's) inherit it.
- User prefs popover (locale + light/dark + accent picker) in the workspace header.
- Prefs persisted in a small PB collection keyed to the user (syncable across devices,
  manageable in-app).

## 3. Backend changes (reusing PB patterns)
- `core/collection_model_base_options.go` — extend `collectionBaseOptions` with the `source` block; validator parity.
- `core/datasource/` (new) — Registry, per-dialect resolvers, REST fetcher, cache; `dbx.NullStringMap`->`[]*Record` hydration (mirror `record_query.go`).
- `apis/record_crud.go` — branch on `collection.datasource.type` -> external executor (read-only); local path untouched.
- `apis/nbx/` (new) — routes: `/buttons/{id}/run`, `/reports/{id}/pdf`, `/export/...`, `/import/...`, `/export/db`, `/import/db`, `/datasources`. Register in `apis/base.go` `NewRouter` (mirror `bindRecordCrudApi`).
- `svcs/io/` (new) — shared import/export engine: CSV/XLSX + SQL-table targets; per-dialect DDL + column mapping; used by both HTTP routes and CLI commands.
- `cmd/nbx/` (new) — cobra commands registered from `plugins/nbx`: `export`, `import`, `export-db`, `import-db` (run against the app instance directly; no auth token; works offline).
- `core/ai/` (new) — `LLMClient` (OpenAI-compatible), `ToolRegistry`, tool-loop engine.
- `core/i18n/` (new) + UI i18n module — dictionaries + `$t()` + user locale store (EN/CZ, extensible).
- `plugins/nbx/register.go` (new) — `Register(app)` wiring `Se.UIExtensions` + hooks; consumed from a new binary.
- Meta collections auto-created at bootstrap (system migration, reuse `RunSystemMigrations`).

## 4. Frontend changes (extension, not fork)
New UI extension dir `ui-ext/nbx/` registering: `app.store.headerLinks` (new nav),
`app.routes.superuserOnly("#/workspace", ...)`, nav per view/dashboard type, and components
`datasView`, `dashboard`, `report`, `button`, `importExport`, `assistantPanel`, `prefsPopover`,
`nbxMap` (OSM multi-marker Leaflet map widget).
Reuse `app.pb.collection(...).getList`, `app.store.collections`, `app.fieldTypes`,
`app.modals.openRecordUpsert`, `watch/store`, CSS under `ui/src/css/_main.css`-style.
i18n: all extension strings go through `app.i18n.$t(key)` (EN/CZ dictionaries). Theme: extension
components consume the shared CSS variables; user prefs applied via `--accentColor`/data-color-scheme.

## 5. Deliverable runnable target
New binary `./cmd/nbx/main.go` — `pocketbase.New()` + `plugins/nbx.Register(app)` + UI
extension -> full app with all features.

## 6. Phases

- **P0 — Scaffold & platform**: `cmd/nbx/main.go` (pocketbase.New + plugins.nbx.Register + UI ext);
  system migration creating `_datasources/_views/_dashboards/_reports/_buttons/_preferences`;
  `source` block in `collectionBaseOptions`.
- **P1 — External datasource engine (read-only)**: `core/datasource/`, per-dialect dbx builders,
  REST fetch + cache, `record_crud.go` routing, `/nbx/datasources` + validation.
  **DoD:** external table browsable/searchable/sortable, no schema sync.
- **P2 — Views**: `_views` config + `datasView` components: grid -> card/kanban -> calendar ->
  gallery -> form. Reuse fieldTypes throughout.
- **P3 — Dashboards, reports, PDF**: widgets (KPI/table/chart/text/**map**), report render,
  server-side HTML->PDF.
- **P4 — Buttons & scripting**: `open_page` (UI-only) then `run_js` via JSVM.
- **P5 — Import/Export**: shared `svcs/io` engine -> CSV + **XLSX via excelize** (streaming,
  both directions) -> **SQL-table targets** (export-db/import-db, HTTP + CLI; auto-create table,
  `--replace`, optional `--create-collection`).
- **P6 — AI assistant**: `core/ai` client + tool registry + tool loop -> `/api/ai/chat` (SSE) ->
  copilot side-panel -> config page (OpenRouter / Ollama / LM Studio verified E2E).
- **P7 — i18n & theming**: prefs collection + i18n module (EN/CZ, `$t`) + per-user theme overrides
  (`--accentColor`/data-color-scheme) + workspace prefs popover.
- **P8 — Docs, tests, validation suites.**

## 7. Key risks / constraints
- **SQLite-isms in core** (`PRAGMA_TABLE_INFO`, `sqlite_master`, WAL, JSON column types, `rowid`)
  — external sources must **bypass** these, which is why we route at `record_crud.go` rather than
  reusing `SaveView`. Kept read-only to avoid cross-dialect DDL.
- **"Reuse not new modules"** tension is sharpest on PDF + AI streaming. Mitigation: small
  swappable `PdfRenderer`; single OpenAI-compatible client for all AI providers. (Excel is a
  deliberate exception — use the standard `excelize` lib rather than a hand-rolled XLSX writer.)
- Bundled SPA is vanilla JS/shablon, **not** Go HTML templates — "reuse PB HTML templates for
  pages" maps to the **UI extension + declarative JSON config** pattern (the Airtable view model),
  not server-side Go templates.
- **i18n scope ceiling:** the bundled PB admin UI has no i18n; only our extension's strings are
  localized (user content stays as typed). Full multilingual admin would require forking `ui/src`.
- **AI prompt-injection:** record content could instruct the assistant. Read-only queries
  auto-run; all schema writes gated behind user confirmation.
- **SQL table export/import:** cross-dialect DDL/quoting and reserved-word quirks; type fidelity
  for JSON-encoded fields (relation/select/json/geoPoint/file); `--create-collection` infers
  best-effort field types (review before use); DSNs hold credentials (plaintext in config for
  now, encryption later via `--encryptionEnv`).

## 8. Decisions (confirmed)
1. F1 external SQL/REST data sources = **read-only** (browse/search/export; no live cross-dialect schema sync). SQL **table export/import** (F5) is a separate, explicit one-shot write/move operation to an external SQL table — the only path that writes to external SQL.
2. Config lives in **PB collections** (`_datasources`, `_views`, `_dashboards`, `_reports`,
   `_buttons`, `_preferences`).
3. **Full frontend:** grid + kanban + calendar + gallery + form, reusing `app.fieldTypes`.
4. PDF via **server-side HTML->PDF renderer** (swappable interface).
5. AI assistant: **queries auto, schema changes approved** (preview-and-confirm).
6. AI chat lives in a **persistent copilot side-panel** (across the workspace).
7. Excel import/export via **`github.com/qax-os/excelize`** (streaming).
8. Per-user locale/theme stored in a **syncable PB collection** (`_preferences`).
9. Multilingual scope = **UI strings only** (EN + CZ initially, extensible); user content as typed.
10. Import/export exposed both via **HTTP and CLI**; SQL-table targets (export-db/import-db) are
    SQL-only (no REST), share one `svcs/io` engine, auto-create missing tables (append by default,
    `--replace` optional), and support `--create-collection` on import.

## 9. Open verification points to resolve at implementation start
- Confirm HTML->PDF renderer lib name (honor lean-on-PB constraint), checked against `go.mod`.
- Whether `.pb.js` hooks reusing `$dbx` can also target external dialects cleanly, or if `run_js` buttons need their own bindings.
