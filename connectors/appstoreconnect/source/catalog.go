package appstoreconnect

import (
	"context"
	"fmt"
	"slices"
)

type resource struct {
	name        string
	displayName string
	cursorCol   string
	defaultOn   bool
}

var resources = []resource{
	{name: "apps", displayName: "Apps", cursorCol: "", defaultOn: true},
	{name: "customer_reviews", displayName: "Customer Reviews", cursorCol: "created_at", defaultOn: true},
	{name: "analytics_reports", displayName: "Analytics Reports", cursorCol: "processing_date", defaultOn: true},
	{name: "sales_reports", displayName: "Sales Reports", cursorCol: "report_date", defaultOn: false},
}

// PlanResources resolves selected resources, keeping optional resources opt-in.
func (s *Source) PlanResources(_ context.Context, names, selectors []string) ([]string, error) {
	selected := append(slices.Clone(names), selectors...)
	if len(selected) == 0 {
		for _, r := range resources {
			if r.defaultOn {
				selected = append(selected, r.name)
			}
		}
	}
	slices.Sort(selected)
	selected = slices.Compact(selected)
	for _, name := range selected {
		if _, err := lookupResource(name); err != nil {
			return nil, err
		}
	}
	return selected, nil
}

func lookupResource(name string) (resource, error) {
	for _, r := range resources {
		if r.name == name {
			return r, nil
		}
	}
	return resource{}, fmt.Errorf("appstoreconnect source: unknown resource %q", name)
}
