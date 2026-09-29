package httpapi

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

//go:embed manifests
var catalogFS embed.FS

// Registry metadata owns release selection; the YAML owns the API contract.
type catalogRegistry struct {
	SchemaVersion  int                       `json:"schema_version"`
	Name           string                    `json:"name"`
	DefaultVersion string                    `json:"default_version"`
	Versions       map[string]catalogVersion `json:"versions"`
}

type catalogVersion struct {
	Maturity filament.ConnectorMaturity `json:"maturity"`
}

type catalogEntry struct {
	name      string
	version   string
	maturity  filament.ConnectorMaturity
	isDefault bool
	data      []byte
}

var catalogName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var catalogRelease = regexp.MustCompile(`^v[1-9][0-9]*$`)

// loadCatalog validates every entry before its caller mutates the registry.
// Reading through fs.FS keeps startup discovery and tests on the same path.
func loadCatalog(files fs.FS) ([]catalogEntry, error) {
	dirs, err := fs.ReadDir(files, "manifests")
	if err != nil {
		return nil, err
	}
	var entries []catalogEntry
	for _, dir := range dirs {
		root := path.Join("manifests", dir.Name())
		if !dir.IsDir() || !catalogName.MatchString(dir.Name()) {
			return nil, fmt.Errorf("%s: expected connector directory", root)
		}
		registryPath := path.Join(root, "registry.json")
		data, err := fs.ReadFile(files, registryPath)
		if err != nil {
			return nil, err
		}
		var registration catalogRegistry
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&registration); err != nil {
			return nil, fmt.Errorf("%s: %w", registryPath, err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("%s: expected one JSON document", registryPath)
		}
		if registration.SchemaVersion != 1 {
			return nil, fmt.Errorf("%s: unsupported schema_version %d", registryPath, registration.SchemaVersion)
		}
		if registration.Name != dir.Name() {
			return nil, fmt.Errorf("%s: name must match directory %q", registryPath, dir.Name())
		}
		if _, ok := registration.Versions[registration.DefaultVersion]; !ok {
			return nil, fmt.Errorf("%s: default_version %q is not listed", registryPath, registration.DefaultVersion)
		}
		versions := make([]string, 0, len(registration.Versions))
		for version := range registration.Versions {
			versions = append(versions, version)
		}
		sort.Strings(versions)
		for _, version := range versions {
			if !catalogRelease.MatchString(version) {
				return nil, fmt.Errorf("%s: invalid connector version %q (want v1, v2, ...)", registryPath, version)
			}
			release := registration.Versions[version]
			switch release.Maturity {
			case filament.MaturityAlpha, filament.MaturityBeta, filament.MaturityStable:
			default:
				return nil, fmt.Errorf("%s: invalid maturity %q for %s", registryPath, release.Maturity, version)
			}
			manifestPath := path.Join(root, version, "manifest.yaml")
			data, err := fs.ReadFile(files, manifestPath)
			if err != nil {
				return nil, err
			}
			parsed, err := manifest.Parse(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", manifestPath, err)
			}
			if parsed.Name != registration.Name {
				return nil, fmt.Errorf("%s: name must match registry name %q", manifestPath, registration.Name)
			}
			if parsed.DisplayName == "" || parsed.Description == "" || parsed.DarkLogoURL == "" || parsed.LightLogoURL == "" {
				return nil, fmt.Errorf("%s: missing catalog presentation metadata", manifestPath)
			}
			entries = append(entries, catalogEntry{name: registration.Name, version: version, maturity: release.Maturity, isDefault: version == registration.DefaultVersion, data: data})
		}
		// Catch forgotten registry entries rather than silently omitting new versions.
		children, err := fs.ReadDir(files, root)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if child.IsDir() {
				if _, ok := registration.Versions[child.Name()]; !ok {
					return nil, fmt.Errorf("%s: unlisted version directory %q", registryPath, child.Name())
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("manifests: catalog is empty")
	}
	return entries, nil
}

func (entry catalogEntry) key() string { return entry.name + "@" + entry.version }

func (entry catalogEntry) source() filament.Source {
	// Parse per instance so mutable manifest maps and slices never cross runs.
	source := NewManifest(entry.data)
	source.name = entry.key()
	source.catalogVersion = entry.version
	return source
}
