package datasource

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/search"
)

// httpClient is the shared HTTP client used for REST datasources.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// RestInterceptor is a seam for customizing outbound REST requests (eg.
// attaching auth headers) and for tests to inject a fake endpoint.
var RestInterceptor func(req *http.Request, ds core.DataSource, cred core.Credential) error

// listREST executes a read-only, paginated list over a REST datasource.
func (r *Registry) listREST(
	collection *core.Collection,
	ds core.DataSource,
	page int,
	perPage int,
	sort string,
	filter string,
) (*search.Result, error) {
	rows, err := fetchRESTRows(ds)
	if err != nil {
		return nil, err
	}

	filtered, err := filterRESTRows(rows, filter)
	if err != nil {
		return nil, err
	}

	sorted := sortRESTRows(filtered, sort)

	total := len(sorted)
	start := (page - 1) * perPage
	if perPage < 0 {
		perPage = 0
	}
	if start > len(sorted) {
		start = len(sorted)
	}
	end := start + perPage
	if perPage == 0 || end > len(sorted) {
		end = len(sorted)
	}

	result, err := core.RecordsFromNullStringMaps(collection, sorted[start:end])
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if perPage > 0 {
		totalPages = total / perPage
		if total%perPage != 0 {
			totalPages++
		}
	}

	return &search.Result{
		Items:      result,
		Page:       page,
		PerPage:    perPage,
		TotalItems: total,
		TotalPages: totalPages,
	}, nil
}

// fetchRESTRows fetches and parses the datasource JSON into a row slice.
func fetchRESTRows(ds core.DataSource) ([]dbx.NullStringMap, error) {
	url := strings.TrimSpace(ds.URL)
	if url == "" {
		return nil, fmt.Errorf("missing datasource url")
	}

	req, err := http.NewRequestWithContext(context.Background(), methodOrDefault(ds.Method), url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range ds.Headers {
		req.Header.Set(k, v)
	}
	if RestInterceptor != nil {
		if err := RestInterceptor(req, ds, core.Credential{}); err != nil {
			return nil, err
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch datasource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datasource returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	objs, err := extractRESTArray(payload, ds.JSONPath)
	if err != nil {
		return nil, err
	}

	result := make([]dbx.NullStringMap, 0, len(objs))
	for _, obj := range objs {
		row := dbx.NullStringMap{}
		for k, v := range obj {
			s, valid := scalarString(v)
			row[strings.TrimSpace(k)] = sql.NullString{
				String: s,
				Valid:  valid,
			}
		}
		result = append(result, row)
	}

	return result, nil
}

func methodOrDefault(method string) string {
	if method == "" {
		return http.MethodGet
	}
	return strings.ToUpper(method)
}

// extractRESTArray extracts the []any array from the payload, optionally
// navigating a dot-path (eg. "data.items").
func extractRESTArray(payload any, jsonPath string) ([]map[string]any, error) {
	current := payload

	if jsonPath != "" {
		for _, part := range strings.Split(jsonPath, ".") {
			obj, ok := current.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid json path %q", jsonPath)
			}
			current, ok = obj[part]
			if !ok {
				return nil, fmt.Errorf("missing json path key %q", part)
			}
		}
	}

	switch t := current.(type) {
	case []any:
		result := make([]map[string]any, 0, len(t))
		for _, item := range t {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, obj)
		}
		return result, nil
	case map[string]any:
		return []map[string]any{t}, nil
	}

	return nil, fmt.Errorf("datasource payload is not an array")
}

// scalarString converts a JSON value to a string representation.
func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case bool:
		if t {
			return "true", true
		}
		return "false", true
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t)), true
		}
		return fmt.Sprintf("%v", t), true
	case json.Number:
		return t.String(), true
	case map[string]any, []any:
		raw, _ := json.Marshal(v)
		return string(raw), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// filterRESTRows applies a naive equality filter on in-memory rows.
func filterRESTRows(rows []dbx.NullStringMap, filter string) ([]dbx.NullStringMap, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return rows, nil
	}

	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("REST filter only supports simple equality")
	}

	col := strings.TrimSpace(parts[0])
	val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

	if col == "" {
		return nil, fmt.Errorf("invalid REST filter column")
	}

	result := make([]dbx.NullStringMap, 0, len(rows))
	for _, row := range rows {
		if row[col].String == val {
			result = append(result, row)
		}
	}

	return result, nil
}

// sortRESTRows applies a naive single-field sort on in-memory rows.
func sortRESTRows(rows []dbx.NullStringMap, sort string) []dbx.NullStringMap {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return rows
	}

	desc := false
	col := sort
	if strings.HasPrefix(col, "-") {
		desc = true
		col = strings.TrimPrefix(col, "-")
	} else {
		col = strings.TrimPrefix(col, "+")
	}

	sorted := append([]dbx.NullStringMap(nil), rows...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a := sorted[j][col].String
			b := sorted[j-1][col].String
			less := a < b
			if desc {
				less = a > b
			}
			if !less {
				break
			}
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	return sorted
}
