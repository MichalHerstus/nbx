// NextBase (nbx) P2 — Views & Workspace.
//
// This file is concatenated by the server into /_/extensions.js and executed as
// a module in the bundled PB admin SPA. It reuses the existing global helpers
// (t, store, watch from shablon; app.* from the bundled SPA).

window.app = window.app || {};
window.app.components = window.app.components || {};
window.app.routes = window.app.routes || {};

const NbxViewsCollection = "_views";

// load the extension stylesheet (served from the extension static dir)
if (!document.querySelector(`link[href*="/extensions/nbx/nbx.css"]`)) {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = app.pb.buildURL("/_/extensions/nbx/nbx.css");
    document.head.appendChild(link);
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function nbxJSON(value, fallback = {}) {
    if (!value || typeof value != "object") {
        return fallback;
    }
    return value;
}

function nbxViewsRecord() {
    return app.store.collections.find((c) => c.id == NbxViewsCollection || c.name == NbxViewsCollection) || null;
}

// canonical JSON stringification for the config field (no trailing spaces)
function nbxDump(value) {
    return JSON.stringify(value, null, 4);
}

function nbxResolveCollection(idOrName) {
    if (!idOrName) {
        return null;
    }
    return (
        app.store.collections.find((c) => c.id == idOrName || c.name == idOrName) || null
    );
}

async function nbxLoadViews() {
    const result = await app.pb.collection(NbxViewsCollection).getList(1, 200, {
        sort: "created",
        requestKey: "nbx.loadViews",
    });
    return result.items;
}

function nbxViewTypeIcon(type) {
    switch (type) {
        case "kanban":
            return "ri-kanban-line";
        case "calendar":
            return "ri-calendar-line";
        case "gallery":
            return "ri-layout-masonry-line";
        case "form":
            return "ri-input-cursor-line";
        case "grid":
        default:
            return "ri-table-line";
    }
}

// ---------------------------------------------------------------------------
// view editor — create / edit a _views record
// ---------------------------------------------------------------------------

const NbxViewTypes = ["grid", "kanban", "calendar", "gallery", "form"];

// default config for a given view type (applied when creating a new view)
function nbxDefaultViewConfig(type, collection) {
    const config = { perPage: 100 };

    switch (type) {
        case "kanban": {
            const groupBy = collection?.fields?.find((f) => f.type == "select" || f.type == "relation")?.name;
            if (groupBy) {
                config.groupBy = groupBy;
            }
            break;
        }
        case "calendar": {
            const dateField = collection?.fields?.find((f) => f.type == "date")?.name;
            if (dateField) {
                config.field = dateField;
            }
            break;
        }
        case "gallery": {
            const fileField = collection?.fields?.find((f) => f.type == "file")?.name;
            if (fileField) {
                config.field = fileField;
            }
            break;
        }
    }

    return config;
}

// Opens a modal for creating/editing a _views record. Returns a promise that
// resolves with the saved record (or null when cancelled).
function nbxViewEditorModal(view = null) {
    const uniqueId = "nbx_view_editor_" + app.utils.randomString();

    const data = store({
        isNew: !view?.id,
        label: view?.label || "",
        sourceCollection: view?.sourceCollection || app.store.activeCollection?.id || "",
        type: view?.type || "grid",
        config: nbxJSON(view?.config),
        isLoading: false,
    });

    let modal;
    let result = null;
    let resolveClose;

    const collectionOptions = () =>
        app.store.collections.map((c) => ({
            value: c.id,
            label: c.label || c.name,
        }));

    function adaptType(type) {
        const col = nbxResolveCollection(data.sourceCollection);
        data.type = type;
        data.config = nbxDefaultViewConfig(type, col);
    }

    async function save(ev) {
        ev?.preventDefault();

        if (!data.sourceCollection) {
            app.toasts.error("Please select a source collection.");
            return;
        }

        data.isLoading = true;
        try {
            const payload = {
                label: data.label || data.sourceCollection,
                sourceCollection: data.sourceCollection,
                type: data.type,
                config: data.config,
            };
            const collection = app.pb.collection(NbxViewsCollection);
            result = data.isNew
                ? await collection.create(payload)
                : await collection.update(view.id, payload);
            app.toasts.success("View saved.");
            app.modals.close(modal, true);
            return result;
        } catch (err) {
            app.checkApiError(err);
            return null;
        } finally {
            data.isLoading = false;
        }
    }

    modal = t.div(
        {
            id: uniqueId,
            className: "modal nbx-view-editor",
            onafterclose: (el) => {
                el?.remove();
                resolveClose?.(result);
            },
            onunmount: () => {
                // no watchers to clean up
            },
        },
        t.header(
            { className: "modal-header" },
            t.h6({ className: "modal-title" }, data.isNew ? "Create view" : "Edit view"),
        ),
        t.div(
            { className: "modal-content" },
            t.form(
                { className: "grid", onsubmit: save },
                t.div(
                    { className: "col-12" },
                    t.label({ for: uniqueId + "_label" }, "Label"),
                    t.input({
                        id: uniqueId + "_label",
                        type: "text",
                        value: () => data.label,
                        placeholder: "My view",
                        oninput: (ev) => (data.label = ev.target.value),
                    }),
                ),
                t.div(
                    { className: "col-12" },
                    t.label({ for: uniqueId + "_source" }, "Source collection"),
                    app.components.select({
                        id: uniqueId + "_source",
                        required: true,
                        value: () => data.sourceCollection,
                        options: collectionOptions(),
                        onchange: (sel) => {
                            data.sourceCollection = sel?.[0]?.value || "";
                            // adapt default config on new views when switching the source
                            if (data.isNew) {
                                adaptType(data.type);
                            }
                        },
                    }),
                ),
                t.div(
                    { className: "col-12" },
                    t.label({ for: uniqueId + "_type" }, "View type"),
                    app.components.select({
                        id: uniqueId + "_type",
                        required: true,
                        value: () => data.type,
                        options: NbxViewTypes.map((t) => ({
                            value: t,
                            label: t.charAt(0).toUpperCase() + t.slice(1),
                        })),
                        onchange: (sel) => {
                            const type = sel?.[0]?.value || "grid";
                            if (data.isNew) {
                                adaptType(type);
                            } else {
                                data.type = type;
                            }
                        },
                    }),
                ),
                t.div(
                    { className: "col-12" },
                    t.label({ for: uniqueId + "_config" }, "Config (advanced)"),
                    t.textarea({
                        id: uniqueId + "_config",
                        rows: 8,
                        spellcheck: "false",
                        className: "code",
                        value: () => nbxDump(data.config),
                        oninput: (ev) => {
                            try {
                                data.config = JSON.parse(ev.target.value);
                            } catch {
                                // leave as-is while editing
                            }
                        },
                    }),
                ),
            ),
        ),
        t.div(
            { className: "modal-footer" },
            t.button(
                { type: "button", className: "btn", onclick: () => app.modals.close(modal) },
                "Cancel",
            ),
            t.button(
                {
                    className: "btn primary",
                    onclick: save,
                    disabled: () => data.isLoading,
                },
                data.isNew ? "Create" : "Save",
            ),
        ),
    );

    return new Promise((resolve) => {
        resolveClose = resolve;
        document.body.appendChild(modal);
        app.modals.open(modal);
    });
}

async function nbxDeleteView(view) {
    if (!view?.id) {
        return;
    }
    await new Promise((resolve) => {
        app.modals.confirm(
            `${"Delete view"} "${view.label || view.id}"?`,
            async () => {
                try {
                    await app.pb.collection(NbxViewsCollection).delete(view.id, { requestKey: "nbx.deleteView" });
                    app.toasts.success("View deleted.");
                    resolve();
                } catch (err) {
                    app.checkApiError(err);
                    resolve();
                }
            },
            () => resolve(),
        );
    });
}

// ---------------------------------------------------------------------------
// datasView — renders a single _views record by its type
// ---------------------------------------------------------------------------

window.app.components.nbxDatasView = function(propsArg = {}) {
    const props = store({ view: {}, collection: {}, records: [], isLoading: false });

    const watchers = app.utils.extendStore(props, propsArg);

    const cfg = () => nbxJSON(props.view?.config);

    async function load() {
        const collection = props.collection;
        if (!collection?.id) {
            props.records = [];
            return;
        }

        props.isLoading = true;
        try {
            const filter = cfg().filter || "";
            const sort = cfg().sort || "";
            const perPage = cfg().perPage || 100;
            const result = await app.pb.collection(collection.id).getList(1, perPage, {
                filter: filter || undefined,
                sort: sort || undefined,
                requestKey: "nbx.dataView",
            });
            props.records = result.items;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            props.isLoading = false;
        }
    }

    watch(() => props.collection?.id, () => load());

    function primaryField() {
        if (cfg().primaryField) {
            return cfg().primaryField;
        }
        return props.collection?.type == "auth"
            ? props.collection.fields.find((f) => f.primaryKey)?.name || "id"
            : "id";
    }

    // --- grid: reuse the bundled records list -----------------------------
    function grid() {
        return t.div(
            { className: "nbx-view nbx-view-grid" },
            app.components.recordsList({
                collection: () => props.collection,
                filter: () => cfg().filter || "",
                sort: () => cfg().sort || "",
            }),
        );
    }

    // --- form: a full page upsert-style editor ----------------------------
    function fieldInputs(record) {
        return props.collection?.fields?.map((field) => {
            if (!app.fieldTypes[field.type]?.input || field.primaryKey) {
                return null;
            }
            return t.div(
                { className: "col-12" },
                app.fieldTypes[field.type].input({
                    get collection() {
                        return props.collection;
                    },
                    get record() {
                        return record;
                    },
                    get field() {
                        return field;
                    },
                }),
            );
        });
    }

    function form() {
        const data = store({
            record: {},
            saving: false,
            get isNew() {
                return !data.record.id;
            },
        });

        return t.div({ className: "nbx-view nbx-view-form" }, () => {
            return t.div(
                { className: "card form-card" },
                t.form(
                    {
                        onsubmit: async (ev) => {
                            ev?.preventDefault();
                            data.saving = true;
                            try {
                                if (data.isNew) {
                                    await app.pb.collection(props.collection.id).create(data.record);
                                } else {
                                    await app.pb.collection(props.collection.id).update(data.record.id, data.record);
                                }
                                app.toasts.success("Saved");
                            } catch (err) {
                                app.checkApiError(err);
                            } finally {
                                data.saving = false;
                            }
                        },
                    },
                    t.div({ className: "grid" }, () => fieldInputs(data.record)),
                    t.div(
                        { className: "form-actions" },
                        t.button(
                            {
                                className: "btn sm primary",
                                type: "submit",
                                disabled: () => data.saving,
                            },
                            data.isNew ? "Create" : "Save",
                        ),
                    ),
                ),
            );
        });
    }

    // --- kanban: group records by a select/relation field ------------------
    function kanban() {
        const groupBy = cfg().groupBy;
        const groupField = props.collection?.fields?.find((f) => f.name == groupBy) || null;
        const groups = store({ data: [] });

        function build() {
            if (!groupField || !props.records.length) {
                groups.data = [];
                return;
            }
            const groupMap = new Map();
            for (const record of props.records) {
                let key = record[groupField.name];
                if (Array.isArray(key)) {
                    key = key.length ? key[0] : "";
                }
                key = (key == null ? "" : String(key)) || "";
                if (!groupMap.has(key)) {
                    groupMap.set(key, []);
                }
                groupMap.get(key).push(record);
            }
            groups.data = [...groupMap.entries()].map(([key, records]) => ({ key, records }));
        }

        watch(() => [props.records, groupBy].join("|"), () => build());

        return t.div({ className: "nbx-view nbx-view-kanban" }, () => {
            if (!groupField) {
                return t.p({ className: "empty-state" }, "Set a groupBy (select/relation) field to use the kanban layout.");
            }
            return t.div(
                { className: "nbx-kanban-columns" },
                groups.data.map((group) =>
                    t.div(
                        { className: "nbx-kanban-column" },
                        t.div({ className: "nbx-kanban-column-head" }, group.key),
                        group.records.map((record) =>
                            t.div(
                                { className: "nbx-kanban-card" },
                                t.div(
                                    { className: "txt-ellipsis" },
                                    app.components.recordSummary?.(record) || record[primaryField()],
                                ),
                            ),
                        ),
                    ),
                ),
            );
        });
    }

    // --- calendar: group records by a date field into a month grid ----------
    function calendar() {
        const dateField = cfg().field || props.collection?.fields?.find((f) => f.type == "date")?.name;
        const data = store({ cursor: new Date(), days: [] });

        function rebuild() {
            const year = data.cursor.getFullYear();
            const month = data.cursor.getMonth();
            const first = new Date(year, month, 1);
            const startDow = (first.getDay() + 6) % 7; // monday-first
            const daysInMonth = new Date(year, month + 1, 0).getDate();
            const list = [];
            for (let i = 0; i < startDow; i++) {
                list.push({ date: null, records: [] });
            }
            for (let d = 1; d <= daysInMonth; d++) {
                list.push({ date: new Date(year, month, d), records: [] });
            }
            if (dateField) {
                for (const record of props.records) {
                    let day = record[dateField];
                    if (!day) {
                        continue;
                    }
                    day = new Date(day);
                    if (day.getFullYear() == year && day.getMonth() == month) {
                        list[startDow + day.getDate() - 1].records.push(record);
                    }
                }
            }
            data.days = list;
        }

        watch(() => [data.cursor.getTime(), props.records.length].join("|"), () => rebuild());

        return t.div({ className: "nbx-view nbx-view-calendar" }, () => {
            const monthLabel = data.cursor.toLocaleDateString("en-CA", { year: "numeric", month: "long" });
            return t.div(
                { className: "nbx-calendar" },
                t.div(
                    { className: "nbx-calendar-toolbar" },
                    t.button({ className: "btn sm", onclick: () => (data.cursor = new Date(data.cursor.getFullYear(), data.cursor.getMonth() - 1, 1)) }, "‹"),
                    t.strong(monthLabel),
                    t.button({ className: "btn sm", onclick: () => (data.cursor = new Date(data.cursor.getFullYear(), data.cursor.getMonth() + 1, 1)) }, "›"),
                ),
                t.div({ className: "nbx-calendar-grid" }, () => {
                    const heads = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
                    return [
                        heads.map((h) => t.div({ className: "nbx-calendar-head" }, h)),
                        data.days.map((day) =>
                            day.date
                                ? t.div(
                                      { className: "nbx-calendar-cell" },
                                      t.span({ className: "nbx-calendar-daynum" }, day.date.getDate()),
                                      day.records.map((r) => t.div({ className: "nbx-calendar-event txt-ellipsis" }, r[primaryField()])),
                                  )
                                : t.div({ className: "nbx-calendar-cell empty" }),
                        ),
                    ];
                }),
            );
        });
    }

    // --- gallery: thumbnails by a file field --------------------------------
    function gallery() {
        const imageField = cfg().field || props.collection?.fields?.find((f) => f.type == "file")?.name;
        return t.div(
            { className: "nbx-view nbx-view-gallery" },
            t.div(
                { className: "nbx-gallery-grid" },
                props.records.map((record) => {
                    const files = record[imageField] || [];
                    const filesList = Array.isArray(files) ? files : files ? [files] : [];
                    const thumb = filesList[0];
                    const url = thumb && thumb.url && app.pb.files.getURL(record, thumb.filename || thumb, { thumb: "300x300" });
                    return t.a(
                        {
                            className: "nbx-gallery-tile",
                            href: "#",
                            onclick: (ev) => {
                                ev?.preventDefault();
                                app.modals.openRecordUpsert(props.collection, record);
                            },
                        },
                        url
                            ? t.img({ src: url, loading: "lazy", alt: "" })
                            : t.div({ className: "nbx-gallery-placeholder" }, app.components.recordSummary?.(record) || record[primaryField()]),
                        t.div({ className: "nbx-gallery-caption txt-ellipsis" }, record[primaryField()]),
                    );
                }),
            ),
        );
    }

    return t.div(
        { className: "nbx-dataview", "data-type": () => props.view?.type || "grid" },
        () => {
            switch (props.view?.type) {
                case "kanban":
                    return kanban();
                case "calendar":
                    return calendar();
                case "gallery":
                    return gallery();
                case "form":
                    return form();
                case "grid":
                default:
                    return grid();
            }
        },
    );
};

// ---------------------------------------------------------------------------
// workspace shell — lists views and offers to open/delete them
// ---------------------------------------------------------------------------

window.app.components.nbxWorkspace = function(propsArg = {}) {
    const props = store({ views: [], isLoading: false });

    const watchers = app.utils.extendStore(props, propsArg);

    async function load() {
        props.isLoading = true;
        try {
            props.views = await nbxLoadViews();
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            props.isLoading = false;
        }
    }

    watch(() => app.store._ready, (r) => r && load());
    watchers?.push(
        watch(() => props.reloadKey, () => load()),
    );

    async function openEditor(view = null) {
        const saved = await nbxViewEditorModal(view);
        if (saved) {
            load();
        }
    }

    async function deleteView(view) {
        await nbxDeleteView(view);
        load();
    }

    return t.div(
        { className: "page" },
        t.div(
            { className: "page-header" },
            t.h1("Workspace"),
            t.button(
                { className: "btn primary", onclick: () => openEditor(null) },
                t.i({ className: "ri-add-line" }),
                "New view",
            ),
        ),
        t.div(
            { className: "grid" },
            t.div(
                { className: "col-12 col-med-6" },
                t.div(
                    { className: "card" },
                    t.div({ className: "card-header" }, t.strong("Views")),
                    () => {
                        if (props.isLoading) {
                            return t.span({ className: "loader sm" });
                        }
                        if (!props.views.length) {
                            return t.p({ className: "empty-state" }, "No views yet. Create one to get started.");
                        }
                        return t.div(
                            { className: "list" },
                            props.views.map((view) => {
                                const col = nbxResolveCollection(view.sourceCollection);
                                return t.div(
                                    { className: "list-item" },
                                    t.i({ className: nbxViewTypeIcon(view.type) }),
                                    t.span(
                                        { className: "txt-ellipsis grow" },
                                        view.label || view.id,
                                        col
                                            ? t.span({ className: "muted" }, " — " + (col.label || col.name))
                                            : t.span({ className: "badge red" }, "missing source"),
                                    ),
                                    t.a(
                                        {
                                            className: "btn sm primary",
                                            href: () => `#/workspace/view/${view.id}`,
                                        },
                                        "Open",
                                    ),
                                    t.button(
                                        { className: "btn sm", onclick: () => openEditor(view) },
                                        t.i({ className: "ri-pencil-line" }),
                                    ),
                                    t.button(
                                        { className: "btn sm", onclick: () => deleteView(view) },
                                        t.i({ className: "ri-delete-bin-line" }),
                                    ),
                                );
                            }),
                        );
                    },
                ),
            ),
        ),
        app.components.nbxReportsSection({}),
    );
};

// NextBase (nbx) P3 — Dashboards, reports & PDF.
//
// Dashboard widgets (kpi/table/chart/text/map), report pages and their PDF
// export are rendered here on top of the server-evaluated widget payload from
// GET /api/nbx/dashboards/{id}/widgets (KPI values/table rows are computed
// server-side; interactive charts/maps use the global uPlot/Leaflet).

const NbxDashboardsCollection = "_dashboards";
const NbxReportsCollection = "_reports";

const NbxWidgetTypes = [
    { value: "kpi", label: "KPI" },
    { value: "table", label: "Data table" },
    { value: "chart", label: "Chart" },
    { value: "text", label: "Text" },
    { value: "map", label: "Map" },
];

async function nbxLoadDashboards() {
    const result = await app.pb.collection(NbxDashboardsCollection).getList(1, 200, {
        sort: "created",
        requestKey: "nbx.loadDashboards",
    });
    return result.items;
}

async function nbxLoadReports() {
    const result = await app.pb.collection(NbxReportsCollection).getList(1, 200, {
        sort: "created",
        requestKey: "nbx.loadReports",
    });
    return result.items;
}

// fetch the server-evaluated widget data for a dashboard
async function nbxFetchWidgets(dashboardId) {
    const res = await app.pb.send("/api/nbx/dashboards/" + dashboardId + "/widgets", {
        method: "GET",
        requestKey: "nbx.dashboardWidgets",
    });
    return res || { data: [] };
}

// KPI display value for a widget based on its aggregate + kpi payload
function nbxKpiDisplay(widget, kpi) {
    if (!kpi) {
        return "—";
    }
    switch (widget.aggregate || "count") {
        case "sum":
            return kpi.sum != null ? kpi.sum.toLocaleString() : "—";
        case "avg":
            return kpi.avg != null ? kpi.avg.toLocaleString(undefined, { maximumFractionDigits: 2 }) : "—";
        case "min":
            return kpi.min != null ? kpi.min.toLocaleString() : "—";
        case "max":
            return kpi.max != null ? kpi.max.toLocaleString() : "—";
        default:
            return kpi.count != null ? kpi.count.toLocaleString() : "—";
    }
}

// --- map widget (OSM multi-marker via the global Leaflet) -------------------
function nbxMapWidget(data) {
    const table = data.table?.rows || [];
    const points = table
        .map((row) => ({
            title: row[0],
            lat: parseFloat((row[1] || "").replace(",", ".")),
            lon: parseFloat((row[2] || "").replace(",", ".")),
        }))
        .filter((p) => Number.isFinite(p.lat) && Number.isFinite(p.lon));

    return t.div(
        {
            className: "nbx-widget-map",
            onmount(el) {
                if (!window.L || !points.length) {
                    el.appendChild(
                        t.div({ className: "nbx-widget-map-empty" }, points.length ? "Maps are unavailable in this browser." : "No geo data to render."),
                    );
                    return;
                }
                const map = L.map(el, { zoomControl: true }).setView([points[0].lat, points[0].lon], 4);
                L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
                    maxZoom: 19,
                    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
                }).addTo(map);

                points.forEach((p) => {
                    const marker = L.marker([p.lat, p.lon]).addTo(map);
                    if (p.title) {
                        marker.bindPopup(p.title);
                    }
                });

                if (points.length > 1) {
                    map.fitBounds(points.map((p) => [p.lat, p.lon]));
                }

                el._nbxMap = map;
            },
            onunmount(el) {
                el._nbxMap?.remove();
            },
        },
        t.div({ className: "nbx-widget-map-loader" }, t.span({ className: "loader sm" })),
    );
}

// --- chart widget (uPlot bar/line from table payload) -----------------------
function nbxChartWidget(data, title) {
    const columns = data.table?.columns || [];
    const rows = data.table?.rows || [];

    return t.div(
        {
            className: "nbx-widget-chart",
            onmount(el) {
                if (!window.uPlot || !rows.length) {
                    el.appendChild(t.div({ className: "nbx-widget-chart-empty" }, rows.length ? "Charts are unavailable in this browser." : "No numeric data to chart."));
                    return;
                }

                const xCol = columns[0] || "x";
                const yCol = columns.length > 1 ? columns[1] : columns[0] || "y";

                const xs = rows.map((_, i) => i);
                const ys = rows.map((row) => {
                    const v = parseFloat(String(row[columns.length > 1 ? 1 : 0] || "").replace(",", "."));
                    return Number.isFinite(v) ? v : 0;
                });

                const opts = {
                    width: el.clientWidth || 320,
                    height: 220,
                    legend: { show: true },
                    scales: { x: { time: false }, y: { min: 0 } },
                    series: [
                        {},
                        {
                            label: yCol,
                            stroke: "var(--accentColor)",
                            width: 2,
                            fill: "rgba(16,85,201,0.15)",
                            points: { show: false },
                        },
                    ],
                    axes: [
                        {
                            values: (self, ticks) => ticks.map((i) => rows[i]?.[0] ?? ""),
                            ticks: { size: 4 },
                            grid: { show: true },
                        },
                        { grid: { show: true } },
                    ],
                };
                el._nbxUplot = new uPlot(opts, [xs, ys], el);
            },
            onunmount(el) {
                el._nbxUplot?.destroy();
            },
        },
        t.div({ className: "nbx-widget-chart-loader" }, t.span({ className: "loader sm" })),
    );
}

// --- table widget ------------------------------------------------------------
function nbxTableWidget(data) {
    const columns = data.table?.columns || [];
    const rows = data.table?.rows || [];
    if (!columns.length) {
        return t.p({ className: "empty-state" }, "No data.");
    }
    return t.div(
        { className: "page-table-wrapper" },
        t.table(
            { className: "responsive-table" },
            t.thead(
                t.tr(columns.map((c) => t.th(c))),
            ),
            t.tbody(
                rows.map((row) =>
                    t.tr(row.map((cell, i) => t.td({ "html-data-name": columns[i] }, cell != null ? String(cell) : ""))),
                ),
            ),
        ),
    );
}

// --- single dashboard widget renderer ---------------------------------------
function nbxWidgetView(entry) {
    const widget = entry.widget || {};
    const cls = ["nbx-widget", "nbx-widget-" + (widget.type || "text")];
    const span = Math.min(12, Math.max(1, widget.span || 4));
    cls.push("span-" + span);

    const body = () => {
        if (entry.error) {
            return t.p({ className: "empty-state" }, "Widget error: " + entry.error);
        }
        switch (widget.type) {
            case "kpi":
                return t.div({ className: "nbx-kpi-value" }, nbxKpiDisplay(widget, entry.kpi));
            case "table":
                return nbxTableWidget(entry);
            case "chart":
                return nbxChartWidget(entry, widget.title);
            case "map":
                return nbxMapWidget(entry);
            case "text":
            default:
                return t.div({ className: "nbx-text-content" }, (entry.notes || []).map((line) => t.p(line)));
        }
    };

    return t.div(
        { className: cls.join(" ") },
        widget.type == "spacer"
            ? null
            : t.div({ className: "nbx-widget-head" }, t.span({ className: "nbx-widget-title" }, widget.title || "")),
        body(),
    );
}

// --- dashboard component ----------------------------------------------------
window.app.components.nbxDashboard = function(propsArg = {}) {
    const props = store({ dashboard: {}, widgets: [], isLoading: false });

    const watchers = app.utils.extendStore(props, propsArg);

    async function load() {
        if (!props.dashboard?.id) {
            props.widgets = [];
            return;
        }
        props.isLoading = true;
        try {
            const payload = await nbxFetchWidgets(props.dashboard.id);
            props.widgets = payload.data || [];
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            props.isLoading = false;
        }
    }

    watch(() => props.dashboard?.id, () => load());
    watchers?.push(watch(() => props.reloadKey, () => load()));

    return t.div(
        { className: "nbx-dashboard" },
        () => {
            if (props.isLoading) {
                return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
            }
            if (!props.widgets.length) {
                return t.p({ className: "empty-state" }, "This dashboard has no widgets yet.");
            }
            return t.div(
                { className: "nbx-widgets-grid" },
                props.widgets.map((entry) => nbxWidgetView(entry)),
            );
        },
    );
};

// --- dashboard & report editor modal -----------------------------------------
function nbxDashboardReportModal(kind, record = null, dashboards = []) {
    const uniqueId = "nbx_dashboard_report_editor_" + app.utils.randomString();
    const isDashboard = kind == "dashboard";
    const collectionName = isDashboard ? NbxDashboardsCollection : NbxReportsCollection;

    const data = store({
        isNew: !record?.id,
        label: record?.label || "",
        dashboard: record?.dashboard || dashboards[0]?.id || "",
        config: nbxJSON(record?.config),
        widgets: isDashboard
            ? (nbxJSON(record?.config)?.widgets || [])
            : [],
    });

    let modal;
    let result = null;
    let resolveClose;

    async function save(ev) {
        ev?.preventDefault();
        if (!data.label) {
            app.toasts.error("Please provide a label.");
            return;
        }
        if (isDashboard) {
            data.config.widgets = data.widgets;
        } else if (!data.dashboard) {
            app.toasts.error("Please select a dashboard.");
            return;
        }

        try {
            const payload = { label: data.label, config: data.config };
            if (!isDashboard) {
                payload.dashboard = data.dashboard;
            }
            const collection = app.pb.collection(collectionName);
            result = data.isNew ? await collection.create(payload) : await collection.update(record.id, payload);
            app.toasts.success("Saved.");
            app.modals.close(modal, true);
            return result;
        } catch (err) {
            app.checkApiError(err);
            return null;
        }
    }

    // add a default widget row for dashboards
    function addWidget() {
        data.widgets.push({
            type: "kpi",
            title: "New metric",
            source: "",
            aggregate: "count",
            span: 4,
        });
    }

    modal = t.div(
        {
            id: uniqueId,
            className: "modal nbx-dash-report-editor",
            onafterclose: (el) => {
                el?.remove();
                resolveClose?.(result);
            },
            onunmount: () => {},
        },
        t.header(
            { className: "modal-header" },
            t.h6({ className: "modal-title" }, data.isNew ? (isDashboard ? "Create dashboard" : "Create report") : (isDashboard ? "Edit dashboard" : "Edit report")),
        ),
        t.div(
            { className: "modal-content" },
            t.form(
                { className: "grid", onsubmit: save },
                t.div(
                    { className: "col-12" },
                    t.label({ for: uniqueId + "_label" }, "Label"),
                    t.input({
                        id: uniqueId + "_label",
                        type: "text",
                        value: () => data.label,
                        oninput: (ev) => (data.label = ev.target.value),
                    }),
                ),
                isDashboard
                    ? null
                    : t.div(
                          { className: "col-12" },
                          t.label({ for: uniqueId + "_dash" }, "Dashboard"),
                          app.components.select({
                              id: uniqueId + "_dash",
                              required: true,
                              value: () => data.dashboard,
                              options: dashboards.map((d) => ({ value: d.id, label: d.label || d.id })),
                              onchange: (sel) => (data.dashboard = sel?.[0]?.value || ""),
                          }),
                      ),
                isDashboard
                    ? t.div(
                          { className: "col-12" },
                          t.div(
                              { className: "flex m-b-sm" },
                              t.span({ className: "txt-sm muted" }, "Widgets"),
                              t.button({ type: "button", className: "btn sm", onclick: addWidget }, t.i({ className: "ri-add-line" }), "Add"),
                          ),
                          () =>
                              data.widgets.map((w, i) =>
                                  t.div(
                                      { className: "nbx-widget-row" },
                                      t.div({ className: "grid" },
                                          t.div({ className: "col-6" },
                                              t.input({
                                                  type: "text",
                                                  placeholder: "Title",
                                                  value: () => w.title || "",
                                                  oninput: (ev) => (w.title = ev.target.value),
                                              }),
                                          ),
                                          t.div({ className: "col-6" },
                                              app.components.select({
                                                  value: () => w.type,
                                                  options: NbxWidgetTypes,
                                                  onchange: (sel) => (w.type = sel?.[0]?.value || "text"),
                                              }),
                                          ),
                                          t.div({ className: "col-12" },
                                              app.components.select({
                                                  value: () => w.source,
                                                  options: app.store.collections
                                                      .filter((c) => c.type == "base" || c.type == "auth")
                                                      .map((c) => ({ value: c.name, label: c.name })),
                                                  placeholder: "Source collection",
                                                  onchange: (sel) => (w.source = sel?.[0]?.value || ""),
                                              }),
                                          ),
                                          w.type == "kpi"
                                              ? t.div({ className: "col-12" },
                                                    app.components.select({
                                                        value: () => w.aggregate,
                                                        options: ["count", "sum", "avg", "min", "max"].map((v) => ({ value: v, label: v })),
                                                        onchange: (sel) => (w.aggregate = sel?.[0]?.value || "count"),
                                                    }),
                                                )
                                              : null,
                                          w.type == "text"
                                              ? t.div({ className: "col-12" },
                                                    t.textarea({
                                                        rows: 3,
                                                        placeholder: "Text content",
                                                        value: () => w.text || "",
                                                        oninput: (ev) => (w.text = ev.target.value),
                                                    }),
                                                )
                                              : null,
                                          w.type == "table" || w.type == "chart"
                                              ? t.div({ className: "col-12" },
                                                    app.components.select({
                                                        value: () => w.field,
                                                        options: [],
                                                        placeholder: "Value field (chart) — leave empty for all table fields",
                                                        onchange: (sel) => (w.field = sel?.[0]?.value || ""),
                                                    }),
                                                )
                                              : null,
                                          w.type == "map"
                                              ? t.div({ className: "col-12" },
                                                    t.input({
                                                        type: "text",
                                                        placeholder: "geoPoint field name",
                                                        value: () => w.field || "",
                                                        oninput: (ev) => (w.field = ev.target.value),
                                                    }),
                                                )
                                              : null,
                                      ),
                                  ),
                              ),
                      )
                    : null,
            ),
        ),
        t.div(
            { className: "modal-footer" },
            t.button({ type: "button", className: "btn", onclick: () => app.modals.close(modal) }, "Cancel"),
            t.button(
                {
                    className: "btn primary",
                    onclick: save,
                },
                data.isNew ? "Create" : "Save",
            ),
        ),
    );

    return new Promise((resolve) => {
        resolveClose = resolve;
        document.body.appendChild(modal);
        app.modals.open(modal);
    });
}

async function nbxDeleteRecord(collectionName, id, label) {
    await new Promise((resolve) => {
        app.modals.confirm(
            `Delete "${label || id}"?`,
            async () => {
                try {
                    await app.pb.collection(collectionName).delete(id, { requestKey: "nbx.delete" });
                    app.toasts.success("Deleted.");
                } catch (err) {
                    app.checkApiError(err);
                }
                resolve();
            },
            () => resolve(),
        );
    });
}

// --- report compose page -----------------------------------------------------
window.app.components.nbxReportPage = function(propsArg = {}) {
    const props = store({ report: {}, dashboard: {}, widgets: [] });

    const watchers = app.utils.extendStore(props, propsArg);

    async function load() {
        if (!props.report?.dashboard) {
            props.dashboard = null;
            props.widgets = [];
            return;
        }
        try {
            props.dashboard = await app.pb.collection(NbxDashboardsCollection).getOne(props.report.dashboard, {
                requestKey: "nbx.reportDashboard",
            });
            const payload = await nbxFetchWidgets(props.dashboard.id);
            props.widgets = payload.data || [];
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }
    }

    watch(() => props.report?.dashboard, () => load());
    watchers?.push(watch(() => props.reloadKey, () => load()));

    function downloadPdf() {
        if (!props.report?.id) {
            return;
        }
        const url = app.pb.buildURL("/api/nbx/reports/" + props.report.id + "/pdf");
        const headers = {};
        const token = app.pb.authStore?.token;
        if (token) {
            headers["Authorization"] = token;
        }
        fetch(url, { method: "GET", headers })
            .then((res) => {
                if (!res.ok) {
                    throw new Error("Failed to generate the report PDF.");
                }
                return res.blob();
            })
            .then((blob) => {
                app.utils.download(window.URL.createObjectURL(blob), (props.report.label || "report") + ".pdf");
            })
            .catch((err) => app.checkApiError(err));
    }

    return t.div(
        { className: "nbx-report-page" },
        t.div(
            { className: "nbx-report-head" },
            t.div({ className: "nbx-report-head-left" }, t.h2(props.report.label || "Report")),
            t.div({ className: "nbx-report-head-actions" }, t.button({ className: "btn primary", onclick: downloadPdf }, t.i({ className: "ri-download-line" }), "PDF")),
        ),
        t.div(
            { className: "nbx-widgets-grid" },
            props.widgets.map((entry) => nbxWidgetView(entry)),
        ),
    );
};

// --- workspace: dashboards + reports cards -----------------------------------
window.app.components.nbxReportsSection = function(propsArg = {}) {
    const props = store({ dashboards: [], reports: [], isLoading: false });

    const watchers = app.utils.extendStore(props, propsArg);

    async function load() {
        props.isLoading = true;
        try {
            props.dashboards = await nbxLoadDashboards();
            props.reports = await nbxLoadReports();
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            props.isLoading = false;
        }
    }

    watch(() => app.store._ready, (r) => r && load());
    watchers?.push(watch(() => props.reloadKey, () => load()));

    async function openEditor(kind, record) {
        const saved = await nbxDashboardReportModal(kind, record, props.dashboards);
        if (saved) {
            load();
        }
    }

    async function remove(collectionName, id, label) {
        await nbxDeleteRecord(collectionName, id, label);
        load();
    }

    const itemActions = (kind, record) =>
        t.div(
            { className: "nbx-list-actions" },
            kind == "report"
                ? t.a({ className: "btn sm primary", href: () => `#/workspace/report/${record.id}` }, "Open")
                : t.a({ className: "btn sm primary", href: () => `#/workspace/dashboard/${record.id}` }, "Open"),
            t.button({ className: "btn sm", onclick: () => openEditor(kind, record) }, t.i({ className: "ri-pencil-line" })),
            t.button({ className: "btn sm", onclick: () => remove(kind == "dashboard" ? NbxDashboardsCollection : NbxReportsCollection, record.id, record.label) }, t.i({ className: "ri-delete-bin-line" })),
        );

    return t.div(
        { className: "grid" },
        t.div(
            { className: "col-12 col-med-6" },
            t.div(
                { className: "card" },
                t.div(
                    { className: "card-header" },
                    t.strong("Dashboards"),
                    t.button({ className: "btn sm", onclick: () => openEditor("dashboard", null) }, t.i({ className: "ri-add-line" })),
                ),
                () => {
                    if (props.isLoading) {
                        return t.span({ className: "loader sm" });
                    }
                    if (!props.dashboards.length) {
                        return t.p({ className: "empty-state" }, "No dashboards yet.");
                    }
                    return t.div(
                        { className: "list" },
                        props.dashboards.map((d) =>
                            t.div({ className: "list-item" }, t.i({ className: "ri-dashboard-line" }), t.span({ className: "txt-ellipsis grow" }, d.label || d.id), itemActions("dashboard", d)),
                        ),
                    );
                },
            ),
        ),
        t.div(
            { className: "col-12 col-med-6" },
            t.div(
                { className: "card" },
                t.div(
                    { className: "card-header" },
                    t.strong("Reports"),
                    t.button({ className: "btn sm", onclick: () => openEditor("report", null) }, t.i({ className: "ri-add-line" })),
                ),
                () => {
                    if (props.isLoading) {
                        return t.span({ className: "loader sm" });
                    }
                    if (!props.reports.length) {
                        return t.p({ className: "empty-state" }, "No reports yet.");
                    }
                    return t.div(
                        { className: "list" },
                        props.reports.map((r) =>
                            t.div({ className: "list-item" }, t.i({ className: "ri-file-chart-line" }), t.span({ className: "txt-ellipsis grow" }, r.label || r.id), itemActions("report", r)),
                        ),
                    );
                },
            ),
        ),
    );
};

// --- routes ------------------------------------------------------------------
app.routes.superuserOnly("#/workspace/dashboard/{id}", async (route) => {
    let dashboard = null;
    try {
        dashboard = await app.pb.collection(NbxDashboardsCollection).getOne(route.params.id, { requestKey: "nbx.dashboard" });
    } catch (err) {
        if (!err.isAbort) {
            app.checkApiError(err);
        }
    }
    if (!dashboard?.id) {
        return t.div({ className: "page" }, t.p({ className: "empty-state" }, "Dashboard not found."));
    }

    app.store.title = dashboard.label || "Dashboard";

    return t.div(
        { className: "page" },
        t.div(
            { className: "page-header" },
            t.h2(dashboard.label || "Dashboard"),
            t.div(
                { className: "page-actions" },
                t.button({ className: "btn", onclick: () => (window.location.hash = "#/workspace") }, "Back"),
            ),
        ),
        app.components.nbxDashboard({ dashboard }),
    );
});

app.routes.superuserOnly("#/workspace/report/{id}", async (route) => {
    let report = null;
    try {
        report = await app.pb.collection(NbxReportsCollection).getOne(route.params.id, { requestKey: "nbx.report" });
    } catch (err) {
        if (!err.isAbort) {
            app.checkApiError(err);
        }
    }
    if (!report?.id) {
        return t.div({ className: "page" }, t.p({ className: "empty-state" }, "Report not found."));
    }

    app.store.title = report.label || "Report";

    return t.div(
        { className: "page" },
        t.div(
            { className: "page-header" },
            t.h2(report.label || "Report"),
            t.div(
                { className: "page-actions" },
                t.button({ className: "btn", onclick: async () => {
                    const dashboards = await nbxLoadDashboards().catch(() => []);
                    const saved = await nbxDashboardReportModal("report", report, dashboards);
                    if (saved?.id) {
                        window.location.reload();
                    }
                } }, t.i({ className: "ri-pencil-line" }), "Edit"),
                t.button({ className: "btn", onclick: () => (window.location.hash = "#/workspace") }, "Back"),
            ),
        ),
        app.components.nbxReportPage({ report }),
    );
});

// add a "Workspace" entry to the top nav
if (!app.store.headerLinks.find((l) => l.href == "#/workspace")) {
    app.store.headerLinks.splice(1, 0, {
        href: "#/workspace",
        icon: "ri-dashboard-3-line",
        label: "Workspace",
    });
}

app.routes.superuserOnly("#/workspace", () => app.components.nbxWorkspace({}));

app.routes.superuserOnly("#/workspace/view/{id}", async (route) => {
    const viewId = route.params.id;

    const load = async () => {
        let view = null;
        try {
            view = await app.pb.collection(NbxViewsCollection).getOne(viewId, {
                requestKey: "nbx.view",
            });
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
            return null;
        }
        return view;
    };

    let view = null;
    view = await load();

    if (!view?.id) {
        return t.div({ className: "page" }, t.p({ className: "empty-state" }, "View not found."));
    }

    const collection = nbxResolveCollection(view.sourceCollection);
    if (!collection?.id) {
        return t.div(
            { className: "page" },
            t.div({ className: "page-header" }, t.h1(view.label || "View")),
            t.p({ className: "empty-state" }, "Missing source collection: " + (view.sourceCollection || "—")),
        );
    }

    app.store.title = view.label || collection.label || collection.name;

    return t.div(
        { className: "page" },
        t.div(
            { className: "page-header" },
            t.h1(view.label || collection.label || collection.name),
            t.div(
                { className: "page-actions" },
                t.button(
                    {
                        className: "btn",
                        onclick: async () => {
                            const saved = await nbxViewEditorModal({ ...view, id: null });
                            if (saved?.id) {
                                window.location.hash = `#/workspace/view/${saved.id}`;
                            }
                        },
                    },
                    t.i({ className: "ri-stack-line" }),
                    "Duplicate",
                ),
                t.button(
                    {
                        className: "btn",
                        onclick: async () => {
                            const saved = await nbxViewEditorModal(view);
                            if (saved?.id) {
                                window.location.hash = `#/workspace/view/${saved.id}`;
                            }
                        },
                    },
                    t.i({ className: "ri-pencil-line" }),
                    "Edit",
                ),
                t.button(
                    {
                        className: "btn danger",
                        onclick: async () => {
                            await nbxDeleteView(view);
                            window.location.hash = "#/workspace";
                        },
                    },
                    t.i({ className: "ri-delete-bin-line" }),
                    "Delete",
                ),
                t.button(
                    {
                        className: "btn",
                        onclick: () => {
                            window.location.hash = "#/workspace";
                        },
                    },
                    "Back",
                ),
            ),
        ),
        app.components.nbxDatasView({ view, collection }),
    );
});