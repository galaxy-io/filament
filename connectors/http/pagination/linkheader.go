package pagination

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// linkHeaderPaginator follows RFC 5988 Link headers. The next URL is whatever
// the server hands back as `<url>; rel="next"`. Stops when no such link is
// present.
type linkHeaderPaginator struct {
	rel string
}

func newLinkHeader(spec manifest.PaginationSpec) (*linkHeaderPaginator, error) {
	rel := spec.Rel
	if rel == "" {
		rel = "next"
	}
	return &linkHeaderPaginator{rel: rel}, nil
}

func (p *linkHeaderPaginator) Initial() State { return State{} }

func (p *linkHeaderPaginator) Apply(req *http.Request, s State) (map[string]any, error) {
	if s.NextURL == "" {
		return nil, nil
	}
	resolved, err := resolveNextURL(req.URL, s.NextURL)
	if err != nil {
		return nil, fmt.Errorf("link_header: %w", err)
	}
	req.URL = resolved
	req.Host = resolved.Host
	return nil, nil
}

func (p *linkHeaderPaginator) Next(resp *http.Response, _ map[string]any, _ int) (State, error) {
	link := resp.Header.Get("Link")
	if link == "" {
		return State{Done: true}, nil
	}
	if next := parseLinkHeader(link, p.rel); next != "" {
		return State{NextURL: next}, nil
	}
	return State{Done: true}, nil
}

// parseLinkHeader returns the URL whose `rel="<targetRel>"` parameter matches.
// Multiple links separated by commas; URL is wrapped in `<...>`.
var linkRE = regexp.MustCompile(`<([^>]+)>\s*;\s*([^,]+)`)

func parseLinkHeader(header, targetRel string) string {
	for _, match := range linkRE.FindAllStringSubmatch(header, -1) {
		urlPart := match[1]
		params := match[2]
		for _, param := range strings.Split(params, ";") {
			kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
			if len(kv) != 2 {
				continue
			}
			if strings.TrimSpace(kv[0]) != "rel" {
				continue
			}
			val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
			if val == targetRel {
				return urlPart
			}
		}
	}
	return ""
}
