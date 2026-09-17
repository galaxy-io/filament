package hubspot

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type resource struct {
	name       string
	objectType string
	modified   string
	defaultOn  bool
}

// CRM objects use one reader. The modification property is an API capability,
// not inferred from a timestamp that happens to appear in a response.
var resources = []resource{
	{"contacts", "0-1", "lastmodifieddate", true},
	{"companies", "0-2", "hs_lastmodifieddate", true},
	{"deals", "0-3", "hs_lastmodifieddate", true},
	{"owners", "", "", true},
	{"calls", "0-48", "hs_lastmodifieddate", true},
	{"meetings", "0-47", "hs_lastmodifieddate", true},
	{"notes", "0-46", "hs_lastmodifieddate", true},
	{"tasks", "0-27", "hs_lastmodifieddate", true},
	{"tickets", "0-5", "hs_lastmodifieddate", false},
	{"emails", "0-49", "hs_lastmodifieddate", false},
	{"products", "0-7", "hs_lastmodifieddate", false},
	{"line_items", "0-8", "hs_lastmodifieddate", false},
	{"quotes", "0-14", "hs_lastmodifieddate", false},
	{"invoices", "0-53", "hs_lastmodifieddate", false},
	{"payments", "0-101", "hs_lastmodifieddate", false},
	{"subscriptions", "0-69", "hs_lastmodifieddate", false},
	{"orders", "0-123", "hs_lastmodifieddate", false},
	{"carts", "0-142", "hs_lastmodifieddate", false},
	{"discounts", "0-84", "hs_lastmodifieddate", false},
	{"fees", "0-85", "hs_lastmodifieddate", false},
	{"taxes", "0-86", "hs_lastmodifieddate", false},
	{"leads", "0-136", "hs_lastmodifieddate", false},
	{"feedback_submissions", "0-19", "hs_lastmodifieddate", false},
	{"goals", "0-74", "hs_lastmodifieddate", false},
	{"communications", "0-18", "hs_lastmodifieddate", false},
	{"postal_mail", "0-116", "hs_lastmodifieddate", false},
	{"appointments", "0-421", "hs_lastmodifieddate", false},
	{"courses", "0-410", "hs_lastmodifieddate", false},
	{"listings", "0-420", "hs_lastmodifieddate", false},
	{"services", "0-162", "hs_lastmodifieddate", false},
	{"projects", "0-970", "hs_lastmodifieddate", false},
	{"crm_users", "0-115", "hs_lastmodifieddate", false},
}

// PlanResources keeps optional resources opt-in, including runs with no selection.
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
	if id, ok := strings.CutPrefix(name, "custom_objects_2_"); ok && id != "" {
		for _, c := range id {
			if c < '0' || c > '9' {
				return resource{}, fmt.Errorf("hubspot source: invalid custom object %q", name)
			}
		}
		return resource{name: name, objectType: "2-" + id, modified: "hs_lastmodifieddate"}, nil
	}
	return resource{}, fmt.Errorf("hubspot source: unknown resource %q", name)
}

func (r resource) path() string {
	if r.name == "owners" {
		return "/crm/owners/" + apiVersion
	}
	return "/crm/objects/" + apiVersion + "/" + r.objectType
}
