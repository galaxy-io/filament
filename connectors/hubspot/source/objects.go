package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

type searchFilter struct {
	Property string `json:"propertyName"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type searchRequest struct {
	Limit        int      `json:"limit"`
	Properties   []string `json:"properties"`
	Sorts        []string `json:"sorts"`
	FilterGroups []struct {
		Filters []searchFilter `json:"filters"`
	} `json:"filterGroups"`
}

func (s *Source) readResource(ctx context.Context, sink arrowbatch.Inlet, r resource, limit int, upper time.Time, cp filament.Checkpoint) error {
	w, err := sink.Builder(r.name, 0, schemaFor(r))
	if err != nil {
		return err
	}
	if r.name == "owners" {
		return s.readOwners(ctx, w, r, limit)
	}
	if cp == nil {
		_, err := s.walkObjects(ctx, r, limit, time.Time{}, upper, func(row object, _ time.Time) error {
			return appendObject(w, r, row, nil)
		})
		return err
	}
	previous, err := planWatermark(r.name, cp)
	if err != nil {
		return err
	}
	if !upper.After(previous) {
		return nil
	}
	searchStart := time.Time{}
	completed := upper
	if !previous.Equal(initialWatermark) {
		lookback, ok := s.lookbacks[r.name]
		if !ok {
			lookback = defaultLookback
		}
		searchStart = previous.Add(-lookback)
		if searchStart.Before(initialWatermark) {
			searchStart = initialWatermark
		}
		completed = previous
	}
	var pending *object
	exhausted, err := s.walkObjects(ctx, r, limit, searchStart, upper, func(row object, selected time.Time) error {
		if pending != nil {
			if err := appendObject(w, r, *pending, nil); err != nil {
				return err
			}
		}
		pending = &row
		if !searchStart.IsZero() && selected.After(completed) {
			completed = selected
		}
		return nil
	})
	if err != nil {
		return err
	}
	if pending == nil {
		return nil // An empty interval retains its checkpoint without a synthetic row.
	}
	var key []string
	if exhausted {
		key = []string{completed.UTC().Format(time.RFC3339Nano)}
	}
	// This final real row carries completion through normal batch acknowledgement.
	// Earlier batches carry no delta. Interrupted reads retain the seeded watermark.
	return appendObject(w, r, *pending, key)
}

func (s *Source) walkObjects(ctx context.Context, r resource, limit int, lower, upper time.Time, visit func(object, time.Time) error) (bool, error) {
	after := ""
	emitted := 0
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		page, err := s.objectPage(ctx, r, after, lower, upper)
		if err != nil {
			return false, err
		}
		if len(page.Results) == 0 {
			return true, nil
		}
		if !lower.IsZero() {
			if err := validateSearchPage(page.Results, after, r.modified, lower, upper); err != nil {
				return false, err
			}
		}
		selected := page.Results
		if limit > 0 && len(selected) > limit-emitted {
			selected = selected[:limit-emitted]
		}
		for start := 0; start < len(selected); start += 100 {
			batch := selected[start:min(start+100, len(selected))]
			rows, err := s.hydrate(ctx, r, batch)
			if err != nil {
				return false, err
			}
			for i, row := range rows {
				var mark time.Time
				if !lower.IsZero() {
					mark, err = modificationTime(batch[i], r.modified)
					if err != nil {
						return false, err
					}
				}
				if err := visit(row, mark); err != nil {
					return false, err
				}
				emitted++
			}
		}
		if limit > 0 && emitted >= limit {
			return false, nil
		}
		if !lower.IsZero() {
			after = page.Results[len(page.Results)-1].ID
			continue // New ID-filtered query; never page a search beyond 10,000 results.
		}
		next := page.Paging.Next.After
		if next == "" {
			return true, nil
		}
		if next == after {
			return false, fmt.Errorf("listing cursor did not advance")
		}
		after = next
	}
}

func (s *Source) objectPage(ctx context.Context, r resource, after string, lower, upper time.Time) (objectPage, error) {
	var page objectPage
	path := r.path()
	method := http.MethodGet
	var body any
	if lower.IsZero() {
		query := url.Values{"limit": {"100"}}
		if after != "" {
			query.Set("after", after)
		}
		path += "?" + query.Encode()
	} else {
		path += "/search"
		method = http.MethodPost
		filters := []searchFilter{
			{r.modified, "GTE", lower.UTC().Format(time.RFC3339Nano)},
			{r.modified, "LT", upper.UTC().Format(time.RFC3339Nano)},
		}
		if after != "" {
			filters = append(filters, searchFilter{"hs_object_id", "GT", after})
		}
		body = searchRequest{
			Limit: 200, Properties: []string{r.modified}, Sorts: []string{"hs_object_id"},
			FilterGroups: []struct {
				Filters []searchFilter `json:"filters"`
			}{{Filters: filters}},
		}
	}
	if err := s.client.request(ctx, r.name, method, path, body, &page); err != nil {
		return page, fmt.Errorf("enumerate records: %w", err)
	}
	if s.client.observe != nil {
		s.client.observe(filament.SourceProgress{Kind: filament.SourceProgressPageFetched, Resource: r.name, Records: int64(len(page.Results)), URI: r.path()})
	}
	return page, nil
}

func (s *Source) hydrate(ctx context.Context, r resource, selected []object) ([]object, error) {
	input := struct {
		Properties []string `json:"properties"`
		Inputs     []struct {
			ID string `json:"id"`
		} `json:"inputs"`
	}{Properties: s.properties[r.name]}
	for _, row := range selected {
		input.Inputs = append(input.Inputs, struct {
			ID string `json:"id"`
		}{ID: row.ID})
	}
	var result struct {
		Status    string            `json:"status"`
		Results   []object          `json:"results"`
		NumErrors int               `json:"numErrors"`
		Errors    []json.RawMessage `json:"errors"`
	}
	if err := s.client.request(ctx, r.name, http.MethodPost, r.path()+"/batch/read", input, &result); err != nil {
		return nil, fmt.Errorf("read properties: %w", err)
	}
	if result.NumErrors > 0 || len(result.Errors) > 0 || (result.Status != "" && result.Status != "COMPLETE") {
		return nil, fmt.Errorf("batch read did not complete successfully (%s)", result.Status)
	}
	byID := make(map[string]object, len(result.Results))
	for _, row := range result.Results {
		if _, duplicate := byID[row.ID]; duplicate {
			return nil, fmt.Errorf("batch read returned duplicate id %s", row.ID)
		}
		byID[row.ID] = row
	}
	rows := make([]object, 0, len(selected))
	for _, selected := range selected {
		row, ok := byID[selected.ID]
		if !ok {
			return nil, fmt.Errorf("batch read omitted id %s; retry the resource", selected.ID)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func validateSearchPage(rows []object, after, modified string, lower, upper time.Time) error {
	previous := new(big.Int)
	if after != "" {
		if _, ok := previous.SetString(after, 10); !ok {
			return fmt.Errorf("invalid search cursor id")
		}
	}
	for _, row := range rows {
		id, ok := new(big.Int).SetString(row.ID, 10)
		if !ok || id.Cmp(previous) <= 0 {
			return fmt.Errorf("search ids did not advance in ascending order")
		}
		mark, err := modificationTime(row, modified)
		if err != nil {
			return err
		}
		if mark.Before(lower) || !mark.Before(upper) {
			return fmt.Errorf("search returned id %s outside the requested interval", row.ID)
		}
		previous = id
	}
	return nil
}

func (s *Source) readOwners(ctx context.Context, w arrowbatch.RowWriter, r resource, limit int) error {
	after := ""
	emitted := 0
	for {
		var page struct {
			Results []json.RawMessage `json:"results"`
			Paging  struct {
				Next struct {
					After string `json:"after"`
				} `json:"next"`
			} `json:"paging"`
		}
		query := url.Values{"limit": {"100"}}
		if after != "" {
			query.Set("after", after)
		}
		if err := s.client.request(ctx, r.name, http.MethodGet, r.path()+"?"+query.Encode(), nil, &page); err != nil {
			return err
		}
		for _, raw := range page.Results {
			var row object
			if err := json.Unmarshal(raw, &row); err != nil {
				return err
			}
			row.Properties = raw // Preserve owner fields and team membership without another schema.
			if err := appendObject(w, r, row, nil); err != nil {
				return err
			}
			emitted++
			if limit > 0 && emitted >= limit {
				return nil
			}
		}
		next := page.Paging.Next.After
		if next == "" {
			return nil
		}
		if next == after {
			return fmt.Errorf("owners cursor did not advance")
		}
		after = next
	}
}
