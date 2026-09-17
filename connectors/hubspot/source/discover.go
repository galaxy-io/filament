package hubspot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament"
)

type property struct {
	Name                    string   `json:"name"`
	DataSensitivity         string   `json:"dataSensitivity"`
	SensitiveDataCategories []string `json:"sensitiveDataCategories"`
}

func (s *Source) loadProperties(ctx context.Context, r resource) error {
	if _, ok := s.properties[r.name]; ok {
		return nil
	}
	var page struct {
		Results []property `json:"results"`
	}
	path := "/crm/properties/" + apiVersion + "/" + r.objectType + "?dataSensitivity=non_sensitive"
	if err := s.client.request(ctx, r.name, http.MethodGet, path, nil, &page); err != nil {
		return fmt.Errorf("hubspot source: %s properties: %w", r.name, err)
	}
	names := make([]string, 0, len(page.Results))
	for _, p := range page.Results {
		if p.Name != "" && len(p.SensitiveDataCategories) == 0 && (p.DataSensitivity == "" || p.DataSensitivity == "non_sensitive") {
			names = append(names, p.Name)
		}
	}
	if r.modified != "" && !slices.Contains(names, r.modified) {
		return fmt.Errorf("hubspot source: %s modification property %q is unavailable", r.name, r.modified)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	s.properties[r.name] = names
	return nil
}

// Discover reports accessible standard objects and account-defined custom objects.
func (s *Source) Discover(ctx context.Context, opts filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.client == nil {
		return filament.DiscoverResult{}, fmt.Errorf("hubspot source: discover before configure")
	}
	if opts.Refresh {
		s.properties = make(map[string][]string)
	}
	var schemas struct {
		Results []struct {
			ObjectTypeID string `json:"objectTypeId"`
			Labels       struct {
				Plural string `json:"plural"`
			} `json:"labels"`
		} `json:"results"`
	}
	err := s.client.request(ctx, "", http.MethodGet, "/crm-object-schemas/"+apiVersion+"/schemas", nil, &schemas)
	if err != nil && !unavailable(err) {
		return filament.DiscoverResult{}, fmt.Errorf("hubspot source: custom schemas: %w", err)
	}
	catalog := append([]resource(nil), resources...)
	labels := make(map[string]string)
	for _, schema := range schemas.Results {
		name := "custom_objects_" + strings.ReplaceAll(schema.ObjectTypeID, "-", "_")
		r, err := lookupResource(name)
		if err != nil {
			return filament.DiscoverResult{}, err
		}
		catalog = append(catalog, r)
		labels[name] = schema.Labels.Plural
	}
	out := filament.DiscoverResult{Resources: make([]filament.Resource, 0, len(catalog))}
	for _, r := range catalog {
		entry := filament.Resource{
			Name: r.name, Selector: r.name, DisplayName: labels[r.name], Selectable: true, PrimaryKey: []string{"id"},
			Metadata: map[string]string{"default_resources": strconv.FormatBool(r.defaultOn)},
		}
		var page objectPage
		err := s.client.request(ctx, r.name, http.MethodGet, r.path()+"?limit=1", nil, &page)
		if err == nil && r.objectType != "" {
			err = s.loadProperties(ctx, r)
		}
		if err != nil {
			if !unavailable(err) {
				return filament.DiscoverResult{}, fmt.Errorf("hubspot source: discover %s: %w", r.name, err)
			}
			entry.Selectable = false
			entry.Metadata["default_resources"] = "false"
			entry.Metadata["unavailable_reason"] = err.Error()
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
