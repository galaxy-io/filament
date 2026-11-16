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

// Products are the Galaxy products a deployment can point at each other,
// each configured by GALAXY_<PRODUCT>_URL.
var Products = []string{"alchemy", "filament"}

// Config is what the server tells the UI at runtime. It is written into
// index.html as JSON, so one UI build serves every deployment.
type Config struct {
	// ProductURLs maps a Galaxy product to where its UI is served. Products
	// link to each other through it and send people back only to these.
	ProductURLs map[string]string `json:"products,omitempty"`
}

// ProductURLsFromEnv reads GALAXY_<PRODUCT>_URL for every product and leaves
// out the unset ones.
func ProductURLsFromEnv(getenv func(string) string) (map[string]string, error) {
	urls := map[string]string{}
	for _, product := range Products {
		name := "GALAXY_" + strings.ToUpper(product) + "_URL"
		raw := getenv(name)
		if raw == "" {
			continue
		}
		u, err := parseProductURL(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		urls[product] = u
	}
	return urls, nil
}

// parseProductURL checks that s is an absolute http or https URL without
// credentials, query or fragment and returns it without a trailing slash.
func parseProductURL(s string) (string, error) {
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
