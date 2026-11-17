package appstoreconnect

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
)

// Discover checks accessible App Store Connect streams and reports their schemas.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.client == nil {
		return filament.DiscoverResult{}, fmt.Errorf("appstoreconnect source: discover before configure")
	}
	var appsPage jsonAPIPage
	appsErr := s.client.requestJSON(ctx, "apps", http.MethodGet, "/v1/apps?limit=1", &appsPage)
	if appsErr != nil && !unavailable(appsErr) {
		return filament.DiscoverResult{}, fmt.Errorf("appstoreconnect source: discover apps: %w", appsErr)
	}
	out := filament.DiscoverResult{Resources: make([]filament.Resource, 0, len(resources))}
	for _, r := range resources {
		entry := filament.Resource{
			Name:        r.name,
			Selector:    r.name,
			DisplayName: r.displayName,
			Selectable:  true,
			PrimaryKey:  []string{"id"},
			Metadata:    map[string]string{"default_resources": strconv.FormatBool(r.defaultOn)},
		}
		var probeErr error
		switch {
		case appsErr != nil:
			probeErr = appsErr
		case r.name == "sales_reports":
			if s.vendorNumber == "" {
				probeErr = &apiError{Status: http.StatusForbidden, Code: "MISSING_VENDOR_NUMBER", Title: "vendor_number is required for sales_reports"}
			} else {
				probeDate := s.now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -2).Format("2006-01-02")
				q := url.Values{
					"filter[frequency]":     {"DAILY"},
					"filter[reportDate]":    {probeDate},
					"filter[reportSubType]": {"SUMMARY"},
					"filter[reportType]":    {"SALES"},
					"filter[vendorNumber]":  {s.vendorNumber},
				}
				_, err := s.client.requestBytes(ctx, r.name, http.MethodGet, "/v1/salesReports?"+q.Encode(), "application/a-gzip, application/json")
				var api *apiError
				if err != nil && (!errors.As(err, &api) || api.Status != http.StatusNotFound) {
					probeErr = err
				}
			}
		case r.name == "analytics_reports" && len(appsPage.Data) > 0:
			var reqPage jsonAPIPage
			path := "/v1/apps/" + appsPage.Data[0].ID + "/analyticsReportRequests?limit=1"
			probeErr = s.client.requestJSON(ctx, r.name, http.MethodGet, path, &reqPage)
		}
		if probeErr != nil {
			if !unavailable(probeErr) {
				return filament.DiscoverResult{}, fmt.Errorf("appstoreconnect source: discover %s: %w", r.name, probeErr)
			}
			entry.Selectable = false
			entry.Metadata["default_resources"] = "false"
			entry.Metadata["unavailable_reason"] = probeErr.Error()
		} else {
			schema := schemaFor(r)
			entry.Schema = &schema
		}
		out.Resources = append(out.Resources, entry)
	}
	slices.SortFunc(out.Resources, func(a, b filament.Resource) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func unavailable(err error) bool {
	var api *apiError
	return errors.As(err, &api) && (api.Status == http.StatusForbidden || api.Status == http.StatusNotFound)
}
