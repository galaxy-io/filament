package pagination

import (
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func bundleBody(links ...map[string]any) map[string]any {
	arr := make([]any, 0, len(links))
	for _, l := range links {
		arr = append(arr, l)
	}
	return map[string]any{"link": arr}
}

// A FHIR searchset Bundle carries page links as {"relation","url"} objects.
// The [key=value] selector follows the "next" relation without hardcoding
// an array position the server is free to reorder.
func TestNextURLSelectorFollowsRelationNext(t *testing.T) {
	p, err := newNextURL(manifest.PaginationSpec{NextURLPath: "link[relation=next].url"})
	if err != nil {
		t.Fatalf("newNextURL: %v", err)
	}
	body := bundleBody(
		map[string]any{"relation": "self", "url": "https://hapi.fhir.org/baseR4/Patient?_count=100"},
		map[string]any{"relation": "next", "url": "https://hapi.fhir.org/baseR4/Patient?_count=100&searchId=abc"},
	)
	state, err := p.Next(nil, body, 200)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if want := "https://hapi.fhir.org/baseR4/Patient?_count=100&searchId=abc"; state.NextURL != want {
		t.Fatalf("NextURL = %q, want %q", state.NextURL, want)
	}
}

// The final page has no "next" link: the selector finds nothing and the walk
// terminates cleanly.
func TestNextURLSelectorTerminatesWithoutNextLink(t *testing.T) {
	p, err := newNextURL(manifest.PaginationSpec{NextURLPath: "link[relation=next].url"})
	if err != nil {
		t.Fatalf("newNextURL: %v", err)
	}
	body := bundleBody(
		map[string]any{"relation": "self", "url": "https://hapi.fhir.org/baseR4/Patient?_count=100"},
	)
	state, err := p.Next(nil, body, 200)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !state.Done {
		t.Fatalf("state = %+v, want Done on a page with no next link", state)
	}
}
