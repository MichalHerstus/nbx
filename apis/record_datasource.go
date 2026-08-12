package apis

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/core/datasource"
	"github.com/pocketbase/pocketbase/tools/search"
)

// datasourceRegistry is the shared external datasource connection registry.
var datasourceRegistry = datasource.NewRegistry()

// datasourceCredential resolves the named credential vault entry referenced by
// the datasource. An empty credential is returned for credential-less sources.
func datasourceCredential(app core.App, ds core.DataSource) core.Credential {
	if ds.CredentialRef == "" {
		return core.Credential{}
	}
	return app.Settings().Nbx.Secrets[ds.CredentialRef]
}

// recordsListExternal handles the records list endpoint for non-local
// (external SQL / REST) datasources. It is strictly read-only.
func recordsListExternal(e *core.RequestEvent, collection *core.Collection) error {
	ds := collection.GetDataSource()

	requestInfo, err := e.RequestInfo()
	if err != nil {
		return firstApiError(err, e.BadRequestError("", err))
	}

	if collection.ListRule == nil && !requestInfo.HasSuperuserAuth() {
		return e.ForbiddenError("Only superusers can perform this action.", nil)
	}

	q := e.Request.URL.Query()

	page := 1
	if v, err := strconv.Atoi(q.Get(search.PageQueryParam)); err == nil && v > 0 {
		page = v
	}

	perPage := search.DefaultPerPage
	if v, err := strconv.Atoi(q.Get(search.PerPageQueryParam)); err == nil && v > 0 && v <= search.MaxPerPage {
		perPage = v
	}

	cred := datasourceCredential(e.App, ds)

	result, err := datasourceRegistry.List(collection, cred, page, perPage, q.Get(search.SortQueryParam), q.Get(search.FilterQueryParam))
	if err != nil {
		return firstApiError(err, e.BadRequestError("Failed to query the external datasource.", err))
	}

	records, _ := result.Items.([]*core.Record)

	event := new(core.RecordsListRequestEvent)
	event.RequestEvent = e
	event.Collection = collection
	event.Records = records
	event.Result = result

	return e.App.OnRecordsListRequest().Trigger(event, func(e *core.RecordsListRequestEvent) error {
		if err := EnrichRecords(e.RequestEvent, e.Records); err != nil {
			return firstApiError(err, e.InternalServerError("Failed to enrich records", err))
		}

		return execAfterSuccessTx(true, e.App, func() error {
			return e.JSON(http.StatusOK, e.Result)
		})
	})
}

// recordsViewExternal handles the record view endpoint for non-local
// datasources (matching by the collection primary key field).
func recordsViewExternal(e *core.RequestEvent, collection *core.Collection) error {
	ds := collection.GetDataSource()

	requestInfo, err := e.RequestInfo()
	if err != nil {
		return firstApiError(err, e.BadRequestError("", err))
	}

	if collection.ViewRule == nil && !requestInfo.HasSuperuserAuth() {
		return e.ForbiddenError("Only superusers can perform this action.", nil)
	}

	recordId := e.Request.PathValue("id")
	cred := datasourceCredential(e.App, ds)

	record, err := datasourceRegistry.Find(collection, cred, recordId)
	if err != nil {
		return firstApiError(err, e.BadRequestError("Failed to query the external datasource.", err))
	}
	if record == nil {
		return e.NotFoundError("", nil)
	}

	event := new(core.RecordRequestEvent)
	event.RequestEvent = e
	event.Collection = collection
	event.Record = record

	return e.App.OnRecordViewRequest().Trigger(event, func(e *core.RecordRequestEvent) error {
		if err := EnrichRecord(e.RequestEvent, e.Record); err != nil {
			return firstApiError(err, e.InternalServerError("Failed to enrich record", err))
		}

		return execAfterSuccessTx(true, e.App, func() error {
			return e.JSON(http.StatusOK, e.Record)
		})
	})
}

// forbidDatasourceWrite rejects mutations (create/update/delete) for
// collections backed by a non-local (external read-only) datasource.
// It returns nil when the mutation is allowed.
func forbidDatasourceWrite(e *core.RequestEvent, collection *core.Collection) error {
	if collection.GetDataSource().IsLocal() {
		return nil
	}
	return e.BadRequestError("The datasource is read-only.", nil)
}
