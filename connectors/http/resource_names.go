package httpapi

import (
	"fmt"

	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func emittedResourceName(res manifest.Resource, parent Capture) (string, error) {
	if res.EmitAs == "" {
		return res.Name, nil
	}
	name, err := template.Render(res.EmitAs, template.Scope{Parent: parent})
	if err != nil {
		return "", fmt.Errorf("render emit_as for resource %q: %w", res.Name, err)
	}
	if name == "" {
		return "", fmt.Errorf("render emit_as for resource %q: empty resource name", res.Name)
	}
	return name, nil
}
