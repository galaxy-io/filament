package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// runtimeConfigElementID is the JSON script in index.html that carries
// Config, read by the standalone host.
const runtimeConfigElementID = "filament-runtime-config"

// Config is what the server tells the UI at runtime. It is written into
// index.html as JSON, so one UI build serves every deployment.
type Config struct {
	// AlchemyURL is where Alchemy's UI is served, such as
	// https://alchemy.example.com. Empty means Filament offers no Alchemy
	// links.
	AlchemyURL string `json:"alchemyUrl,omitempty"`
}

// ParseAlchemyURL checks that s is an absolute http or https URL without
// credentials, query or fragment and returns it without a trailing slash.
// Empty stays empty.
func ParseAlchemyURL(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	isBase := (u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != "" && u.User == nil && u.Opaque == "" &&
		u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
	if !isBase {
		return "", fmt.Errorf("%q is not an absolute http or https URL without query or fragment", s)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// renderIndex writes cfg into index.html as a JSON script before </head>.
// json.Marshal escapes <, > and &, so the JSON cannot close the script.
func renderIndex(index []byte, cfg Config) ([]byte, error) {
	head := []byte("</head>")
	if !bytes.Contains(index, head) {
		return nil, errors.New("index.html has no </head> to write the runtime config before")
	}
	config, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	script := fmt.Appendf(nil, `<script type="application/json" id="%s">%s</script>`, runtimeConfigElementID, config)
	return bytes.Replace(index, head, append(script, head...), 1), nil
}
