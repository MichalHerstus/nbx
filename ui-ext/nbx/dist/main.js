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
    );
};

// ---------------------------------------------------------------------------
// routes + nav
// ---------------------------------------------------------------------------

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