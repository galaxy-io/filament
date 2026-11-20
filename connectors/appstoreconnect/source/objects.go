package appstoreconnect

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

const (
	defaultSalesBackfillDays = 30
	// salesRetryWindowDays bounds how long a missing daily sales report keeps the checkpoint pinned.
	salesRetryWindowDays = 7
)

func (s *Source) readResource(ctx context.Context, sink arrowbatch.Inlet, r resource, limit int, upper time.Time, cp filament.Checkpoint) error {
	w, err := sink.Builder(r.name, 0, schemaFor(r))
	if err != nil {
		return err
	}
	if r.name == "apps" {
		return s.readApps(ctx, w, r, limit)
	}
	if r.name == "sales_reports" || r.name == "analytics_reports" {
		upper = upper.UTC().Truncate(24 * time.Hour)
	}
	if cp == nil {
		_, _, err := s.walkResource(ctx, r, limit, time.Time{}, upper, func(row recordRow) error {
			return appendRecord(w, r, row, nil)
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
	var pending *recordRow
	exhausted, capTime, err := s.walkResource(ctx, r, limit, searchStart, upper, func(row recordRow) error {
		if pending != nil {
			if err := appendRecord(w, r, *pending, nil); err != nil {
				return err
			}
		}
		rowCopy := row
		pending = &rowCopy
		if !searchStart.IsZero() && row.cursorTime.After(completed) {
			completed = row.cursorTime
		}
		return nil
	})
	if err != nil {
		return err
	}
	if pending == nil {
		return nil
	}
	if !capTime.IsZero() && completed.After(capTime) {
		completed = capTime
		if completed.Before(previous) {
			completed = previous
		}
	}
	var key []string
	if exhausted {
		key = []string{completed.UTC().Format(time.RFC3339Nano)}
	}
	return appendRecord(w, r, *pending, key)
}

func (s *Source) walkResource(ctx context.Context, r resource, limit int, lower, upper time.Time, visit func(recordRow) error) (bool, time.Time, error) {
	switch r.name {
	case "customer_reviews":
		exhausted, err := s.walkCustomerReviews(ctx, r, limit, lower, upper, visit)
		return exhausted, time.Time{}, err
	case "analytics_reports":
		exhausted, err := s.walkAnalyticsReports(ctx, r, limit, lower, upper, visit)
		return exhausted, time.Time{}, err
	case "sales_reports":
		return s.walkSalesReports(ctx, r, limit, lower, upper, visit)
	default:
		return false, time.Time{}, fmt.Errorf("unsupported resource %s", r.name)
	}
}

func (s *Source) listAppIDs(ctx context.Context, resourceName string) ([]string, error) {
	if len(s.appIDs) > 0 {
		return slices.Clone(s.appIDs), nil
	}
	var ids []string
	nextPath := "/v1/apps?limit=200"
	for nextPath != "" {
		var page jsonAPIPage
		if err := s.client.requestJSON(ctx, resourceName, nextPath, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Data {
			if item.ID != "" {
				ids = append(ids, item.ID)
			}
		}
		next, err := normalizeNextLink(page.Links.Next, nextPath)
		if err != nil {
			return nil, err
		}
		nextPath = next
	}
	return ids, nil
}

func (s *Source) readApps(ctx context.Context, w arrowbatch.RowWriter, r resource, limit int) error {
	nextPath := "/v1/apps?limit=200"
	if len(s.appIDs) > 0 {
		nextPath += "&filter[id]=" + url.QueryEscape(strings.Join(s.appIDs, ","))
	}
	emitted := 0
	for nextPath != "" {
		var page jsonAPIPage
		if err := s.client.requestJSON(ctx, r.name, nextPath, &page); err != nil {
			return err
		}
		if s.client.observe != nil {
			s.client.observe(filament.SourceProgress{Kind: filament.SourceProgressPageFetched, Resource: r.name, Records: int64(len(page.Data)), URI: "/v1/apps"})
		}
		for _, item := range page.Data {
			if len(s.appIDs) > 0 && !slices.Contains(s.appIDs, item.ID) {
				continue
			}
			row := recordRow{
				id:         item.ID,
				attributes: item.Attributes,
			}
			if err := appendRecord(w, r, row, nil); err != nil {
				return err
			}
			emitted++
			if limit > 0 && emitted >= limit {
				return nil
			}
		}
		next, err := normalizeNextLink(page.Links.Next, nextPath)
		if err != nil {
			return err
		}
		nextPath = next
	}
	return nil
}

func (s *Source) walkCustomerReviews(ctx context.Context, r resource, limit int, lower, upper time.Time, visit func(recordRow) error) (bool, error) {
	appIDs, err := s.listAppIDs(ctx, r.name)
	if err != nil {
		return false, err
	}
	emitted := 0
	for _, appID := range appIDs {
		nextPath := "/v1/apps/" + url.PathEscape(appID) + "/customerReviews?limit=200&sort=-createdDate"
		for nextPath != "" {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			var page jsonAPIPage
			if err := s.client.requestJSON(ctx, r.name, nextPath, &page); err != nil {
				return false, err
			}
			if s.client.observe != nil {
				s.client.observe(filament.SourceProgress{Kind: filament.SourceProgressPageFetched, Resource: r.name, Records: int64(len(page.Data)), URI: "/v1/apps/" + appID + "/customerReviews"})
			}
			reachedOlder := false
			for _, item := range page.Data {
				var attrs struct {
					CreatedDate string `json:"createdDate"`
				}
				if err := json.Unmarshal(item.Attributes, &attrs); err != nil || attrs.CreatedDate == "" {
					return false, fmt.Errorf("review %s missing createdDate", item.ID)
				}
				created, err := time.Parse(time.RFC3339Nano, attrs.CreatedDate)
				if err != nil {
					return false, fmt.Errorf("review %s invalid createdDate %q: %w", item.ID, attrs.CreatedDate, err)
				}
				created = created.UTC()
				if !created.Before(upper) {
					continue
				}
				if !lower.IsZero() && created.Before(lower) {
					reachedOlder = true
					break
				}
				row := recordRow{
					id:         item.ID,
					appID:      appID,
					cursorTime: created,
					attributes: item.Attributes,
				}
				if err := visit(row); err != nil {
					return false, err
				}
				emitted++
				if limit > 0 && emitted >= limit {
					return false, nil
				}
			}
			if reachedOlder {
				break
			}
			next, err := normalizeNextLink(page.Links.Next, nextPath)
			if err != nil {
				return false, err
			}
			nextPath = next
		}
	}
	return true, nil
}

func (s *Source) walkAnalyticsReports(ctx context.Context, r resource, limit int, lower, upper time.Time, visit func(recordRow) error) (bool, error) {
	appIDs, err := s.listAppIDs(ctx, r.name)
	if err != nil {
		return false, err
	}
	emitted := 0
	for _, appID := range appIDs {
		reqPath := "/v1/apps/" + url.PathEscape(appID) + "/analyticsReportRequests?limit=200"
		for reqPath != "" {
			var reqPage jsonAPIPage
			if err := s.client.requestJSON(ctx, r.name, reqPath, &reqPage); err != nil {
				return false, err
			}
			for _, reqObj := range reqPage.Data {
				done, err := s.walkAnalyticsRequestReports(ctx, r.name, appID, reqObj.ID, limit, &emitted, lower, upper, visit)
				if err != nil || !done {
					return done, err
				}
			}
			nextReq, err := normalizeNextLink(reqPage.Links.Next, reqPath)
			if err != nil {
				return false, err
			}
			reqPath = nextReq
		}
	}
	return true, nil
}

func (s *Source) walkAnalyticsRequestReports(ctx context.Context, resName, appID, requestID string, limit int, emitted *int, lower, upper time.Time, visit func(recordRow) error) (bool, error) {
	repPath := "/v1/analyticsReportRequests/" + url.PathEscape(requestID) + "/reports?limit=200"
	for repPath != "" {
		var repPage jsonAPIPage
		if err := s.client.requestJSON(ctx, resName, repPath, &repPage); err != nil {
			return false, err
		}
		for _, repObj := range repPage.Data {
			var repAttrs struct {
				Category string `json:"category"`
				Name     string `json:"name"`
			}
			_ = json.Unmarshal(repObj.Attributes, &repAttrs)
			done, err := s.walkAnalyticsReportInstances(ctx, resName, appID, repObj.ID, repAttrs.Category, repAttrs.Name, limit, emitted, lower, upper, visit)
			if err != nil || !done {
				return done, err
			}
		}
		nextRep, err := normalizeNextLink(repPage.Links.Next, repPath)
		if err != nil {
			return false, err
		}
		repPath = nextRep
	}
	return true, nil
}

func (s *Source) walkAnalyticsReportInstances(ctx context.Context, resName, appID, reportID, category, name string, limit int, emitted *int, lower, upper time.Time, visit func(recordRow) error) (bool, error) {
	instPath := "/v1/analyticsReports/" + url.PathEscape(reportID) + "/instances?filter[granularity]=DAILY&limit=200"
	for instPath != "" {
		var instPage jsonAPIPage
		if err := s.client.requestJSON(ctx, resName, instPath, &instPage); err != nil {
			return false, err
		}
		for _, instObj := range instPage.Data {
			var instAttrs struct {
				ProcessingDate string `json:"processingDate"`
			}
			_ = json.Unmarshal(instObj.Attributes, &instAttrs)
			procDate, err := parseDateOrRFC3339(instAttrs.ProcessingDate)
			if err != nil {
				return false, fmt.Errorf("instance %s invalid processingDate %q: %w", instObj.ID, instAttrs.ProcessingDate, err)
			}
			if !procDate.Before(upper) || (!lower.IsZero() && procDate.Before(lower)) {
				continue
			}
			done, err := s.walkAnalyticsInstanceSegments(ctx, resName, appID, instObj.ID, category, name, procDate, limit, emitted, visit)
			if err != nil || !done {
				return done, err
			}
		}
		nextInst, err := normalizeNextLink(instPage.Links.Next, instPath)
		if err != nil {
			return false, err
		}
		instPath = nextInst
	}
	return true, nil
}

func (s *Source) walkAnalyticsInstanceSegments(ctx context.Context, resName, appID, instanceID, category, name string, procDate time.Time, limit int, emitted *int, visit func(recordRow) error) (bool, error) {
	segPath := "/v1/analyticsReportInstances/" + url.PathEscape(instanceID) + "/segments?limit=200"
	for segPath != "" {
		var segPage jsonAPIPage
		if err := s.client.requestJSON(ctx, resName, segPath, &segPage); err != nil {
			return false, err
		}
		for _, segObj := range segPage.Data {
			var segAttrs struct {
				URL string `json:"url"`
			}
			_ = json.Unmarshal(segObj.Attributes, &segAttrs)
			if segAttrs.URL == "" {
				continue
			}
			rows, err := s.downloadGzipTSV(ctx, resName, segAttrs.URL)
			if err != nil {
				return false, err
			}
			procDateStr := procDate.UTC().Format("2006-01-02")
			for _, metricsMap := range rows {
				rawMetrics, _ := json.Marshal(metricsMap)
				row := recordRow{
					id:         fmt.Sprintf("%s:%s:%s:%s", appID, name, procDateStr, analyticsDimensionHash(metricsMap)),
					appID:      appID,
					category:   category,
					name:       name,
					cursorTime: procDate,
					attributes: rawMetrics,
				}
				if err := visit(row); err != nil {
					return false, err
				}
				*emitted++
				if limit > 0 && *emitted >= limit {
					return false, nil
				}
			}
		}
		nextSeg, err := normalizeNextLink(segPage.Links.Next, segPath)
		if err != nil {
			return false, err
		}
		segPath = nextSeg
	}
	return true, nil
}

func (s *Source) walkSalesReports(ctx context.Context, r resource, limit int, lower, upper time.Time, visit func(recordRow) error) (bool, time.Time, error) {
	endDay := upper.UTC().Truncate(24 * time.Hour)
	startDay := endDay.AddDate(0, 0, -defaultSalesBackfillDays)
	if !s.startDate.IsZero() {
		startDay = s.startDate.UTC().Truncate(24 * time.Hour)
	}
	if !lower.IsZero() && !lower.Equal(initialWatermark) {
		startDay = lower.UTC().Truncate(24 * time.Hour)
	}
	emitted := 0
	var earliestMissing time.Time
	for day := startDay; day.Before(endDay); day = day.AddDate(0, 0, 1) {
		if err := ctx.Err(); err != nil {
			return false, earliestMissing, err
		}
		dateStr := day.Format("2006-01-02")
		query := url.Values{
			"filter[frequency]":     {"DAILY"},
			"filter[reportDate]":    {dateStr},
			"filter[reportSubType]": {"SUMMARY"},
			"filter[reportType]":    {"SALES"},
			"filter[vendorNumber]":  {s.vendorNumber},
		}
		path := "/v1/salesReports?" + query.Encode()
		rows, err := s.downloadGzipTSV(ctx, r.name, path)
		if err != nil {
			var api *apiError
			if errors.As(err, &api) && api.Status == http.StatusNotFound {
				if earliestMissing.IsZero() && !day.Before(endDay.AddDate(0, 0, -salesRetryWindowDays)) {
					earliestMissing = day
				}
				continue // Report not yet available or no sales for this date.
			}
			return false, earliestMissing, err
		}
		for _, metricsMap := range rows {
			rawMetrics, _ := json.Marshal(metricsMap)
			beginDate := metricsMap["Begin Date"]
			if beginDate == "" {
				beginDate = dateStr
			}
			compositeID := strings.Join([]string{
				metricsMap["Provider"],
				metricsMap["SKU"],
				beginDate,
				metricsMap["Country Code"],
				metricsMap["Product Type Identifier"],
			}, ":")
			row := recordRow{
				id:         compositeID,
				appID:      metricsMap["SKU"],
				category:   metricsMap["Country Code"],
				name:       metricsMap["Product Type Identifier"],
				cursorTime: day,
				attributes: rawMetrics,
			}
			if err := visit(row); err != nil {
				return false, earliestMissing, err
			}
			emitted++
			if limit > 0 && emitted >= limit {
				return false, earliestMissing, nil
			}
		}
	}
	return true, earliestMissing, nil
}

func (s *Source) downloadGzipTSV(ctx context.Context, resourceName, pathOrURL string) ([]map[string]string, error) {
	data, err := s.client.requestBytes(ctx, resourceName, pathOrURL, "application/a-gzip, application/json")
	if err != nil {
		return nil, err
	}
	var reader io.Reader = bytes.NewReader(data)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("decompress gzip report: %w", err)
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}
	tsv := csv.NewReader(reader)
	tsv.Comma = '\t'
	tsv.LazyQuotes = true
	tsv.FieldsPerRecord = -1
	headers, err := tsv.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read report header: %w", err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	var out []map[string]string
	for {
		record, err := tsv.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read report row: %w", err)
		}
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			if h != "" && i < len(record) {
				row[h] = strings.TrimSpace(record[i])
			}
		}
		out = append(out, row)
	}
	if s.client.observe != nil {
		s.client.observe(filament.SourceProgress{Kind: filament.SourceProgressPageFetched, Resource: resourceName, Records: int64(len(out)), Bytes: int64(len(data)), URI: redactedURI(pathOrURL)})
	}
	return out, nil
}

// redactedURI removes the query string, which can carry signed download credentials.
func redactedURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.RawQuery = ""
	return u.String()
}

func normalizeNextLink(next, current string) (string, error) {
	if next == "" {
		return "", nil
	}
	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
		u, err := url.Parse(next)
		if err != nil {
			return "", fmt.Errorf("invalid pagination link: %w", err)
		}
		next = u.RequestURI()
	}
	if next == current {
		return "", fmt.Errorf("listing cursor did not advance")
	}
	return next, nil
}

func parseDateOrRFC3339(val string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported date format %q", val)
}

// analyticsDimensionHash computes a 128-bit SHA-256 hex digest over only the dimension columns
// of an analytics report row so restated numeric measure values upsert cleanly in place.
func analyticsDimensionHash(metricsMap map[string]string) string {
	dims := make(map[string]string, len(metricsMap))
	for k, v := range metricsMap {
		if isAnalyticsMeasureColumn(k, v) {
			continue
		}
		dims[k] = v
	}
	if len(dims) == 0 {
		dims = metricsMap
	}
	rawDims, _ := json.Marshal(dims)
	sum := sha256.Sum256(rawDims)
	return hex.EncodeToString(sum[:16])
}

func isAnalyticsMeasureColumn(colName, val string) bool {
	lower := strings.ToLower(strings.TrimSpace(colName))
	for _, idSuffix := range []string{"id", "date", "territory", "device", "platform", "version", "type", "source", "referrer", "campaign", "page", "locale", "region", "country", "code", "name", "title", "state", "status", "category"} {
		if lower == idSuffix || strings.HasSuffix(lower, " "+idSuffix) || strings.HasSuffix(lower, "_"+idSuffix) {
			return false
		}
	}
	trimmed := strings.TrimSpace(strings.TrimSuffix(val, "%"))
	if trimmed == "" {
		return false
	}
	_, err := strconv.ParseFloat(trimmed, 64)
	return err == nil
}
