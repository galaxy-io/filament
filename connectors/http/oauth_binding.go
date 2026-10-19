package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// OAuthRefreshConfig binds managed manifest grants to the saved connection's
// refresh secret. Binding components deliberately exclude the rotating token.
func (s *Source) OAuthRefreshConfig(cfg filament.Config) (field, binding string) {
	c, err := s.connectorForConfig(cfg)
	if err != nil {
		return "", ""
	}
	spec := selectedAuth(c.manifest.Connection.Auth, c.creds)
	if spec.Type != "oauth2_cc" || spec.Params["managed_refresh"] != true {
		return "", ""
	}
	ref, _ := spec.Params["refresh_token"].(string)
	field = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(ref, "{{"), "}}"))
	field = strings.TrimPrefix(field, "config.")
	if c.creds[field] == "" {
		return "", ""
	}
	components, _ := spec.Params["binding"].([]string)
	values := make([]string, len(components))
	for i, component := range components {
		values[i], err = template.Render(component, template.Scope{Config: c.creds})
		if err != nil {
			return "", ""
		}
	}
	encoded, _ := json.Marshal(values)
	sum := sha256.Sum256(encoded)
	return field, hex.EncodeToString(sum[:])
}

func selectedAuth(spec manifest.AuthSpec, creds map[string]string) manifest.AuthSpec {
	if spec.Type != "select" {
		return spec
	}
	field, _ := spec.Params["field"].(string)
	cases, _ := spec.Params["cases"].(map[string]any)
	choice, _ := cases[creds[field]].(map[string]any)
	kind, _ := choice["type"].(string)
	params, _ := choice["params"].(map[string]any)
	return selectedAuth(manifest.AuthSpec{Type: kind, Params: params}, creds)
}
