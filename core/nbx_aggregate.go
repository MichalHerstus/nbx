package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/tools/search"
)

// NbxAggregate holds the result of a single KPI aggregation over a collection.
type NbxAggregate struct {
	// Count is the number of matching records.
	Count int `json:"count"`

	// Sum, Avg, Min, Max are the numeric results (nil when not applicable
	// or when there are no numeric values).
	Sum *float64 `json:"sum,omitempty"`
	Avg *float64 `json:"avg,omitempty"`
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// AggregateLocal computes a KPI aggregation over records of a LOCAL
// (SQLite-backed) collection, honouring the provided search.FilterData filter.
//
// It returns an error for collections backed by an external datasource - for
// those the caller should use the datasource registry instead (see F1).
func AggregateLocal(
	app App,
	collection *Collection,
	filter string,
	field string,
) (*NbxAggregate, error) {
	if !collection.GetDataSource().IsLocal() {
		return nil, fmt.Errorf("aggregate is not supported for external datasources")
	}

	records, err := queryLocalRecords(app, collection, filter)
	if err != nil {
		return nil, err
	}

	out := &NbxAggregate{Count: len(records)}

	if field != "" {
		nums := numericValues(records, field)
		if len(nums) > 0 {
			var sum float64
			min := nums[0]
			max := nums[0]
			for _, n := range nums {
				sum += n
				if n < min {
					min = n
				}
				if n > max {
					max = n
				}
			}
			avg := sum / float64(len(nums))
			out.Sum = &sum
			out.Avg = &avg
			out.Min = &min
			out.Max = &max
		}
	}

	return out, nil
}

// queryLocalRecords returns the records of a local collection applying the
// given search.FilterData filter. Non-superuser visibility rules are skipped
// because aggregation runs server-side for authorized (superuser) reports.
func queryLocalRecords(app App, collection *Collection, filter string) ([]*Record, error) {
	// allowHiddenFields=true on the resolver also disables the collection
	// list-rule join, so aggregation is not constrained by public visibility.
	requestInfo := &RequestInfo{Method: "GET"}

	query := app.RecordQuery(collection)
	resolver := NewRecordFieldResolver(app, collection, requestInfo, true)
	resolver.SetAllowHiddenFields(true)

	filter = strings.TrimSpace(filter)
	if filter != "" {
		expr, err := search.FilterData(filter).BuildExpr(resolver)
		if err != nil {
			return nil, err
		}
		query.AndWhere(expr)
		if err := resolver.UpdateQuery(query); err != nil {
			return nil, err
		}
	}

	records := []*Record{}
	if err := query.All(&records); err != nil {
		return nil, err
	}
	return records, nil
}

// numericValues extracts the numeric values of a record field across the
// provided records (skipping empty/non-numeric entries).
func numericValues(records []*Record, field string) []float64 {
	if len(records) == 0 || field == "" {
		return nil
	}
	seen := 0
	for _, r := range records {
		raw := r.Get(field)
		if raw == nil {
			continue
		}
		switch v := raw.(type) {
		case float64:
			if !math.IsNaN(v) && math.IsInf(v, 0) == false {
				seen++
			}
		case int:
			seen++
		case int64:
			seen++
		case string:
			if _, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				seen++
			}
		}
	}
	if seen == 0 {
		return nil
	}

	nums := make([]float64, 0, seen)
	for _, r := range records {
		raw := r.Get(field)
		if raw == nil {
			continue
		}
		switch v := raw.(type) {
		case float64:
			if !math.IsNaN(v) && math.IsInf(v, 0) == false {
				nums = append(nums, v)
			}
		case int:
			nums = append(nums, float64(v))
		case int64:
			nums = append(nums, float64(v))
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				nums = append(nums, f)
			}
		}
	}
	return nums
}