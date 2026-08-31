// Package contexts manages named CLI targets independently from pipeline
// configuration and authentication credentials.
package contexts

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultContextName = "local"

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Kind identifies how a named target is reached.
type Kind string

const (
	// KindLocal selects a local YAML document and in-process runtime.
	KindLocal Kind = "local"
	// KindRemote selects a deployed Filament service.
	KindRemote Kind = "remote"
)

// Target contains the non-secret settings needed to construct a target
// adapter. AuthProfile is a credential-store key, never a bearer token.
type Target struct {
	Kind        Kind   `yaml:"kind"`
	ConfigPath  string `yaml:"config,omitempty"`
	Endpoint    string `yaml:"endpoint,omitempty"`
	Tenant      string `yaml:"tenant,omitempty"`
	AuthProfile string `yaml:"auth_profile,omitempty"`
}

// NamedTarget associates a target with its context name and selection state.
type NamedTarget struct {
	Name    string
	Current bool
	Target  Target
}

// Document is the persisted, secret-free context registry.
type Document struct {
	Current  string            `yaml:"current"`
	Contexts map[string]Target `yaml:"contexts"`
}

// Store atomically persists a context registry.
type Store struct {
	Path string
}

// Registry resolves and changes contexts while maintaining the built-in local
// context available without setup.
type Registry struct {
	store            Store
	defaultLocalPath string
}

// NewRegistry constructs a registry with a default local target.
func NewRegistry(store Store, defaultLocalPath string) *Registry {
	return &Registry{store: store, defaultLocalPath: defaultLocalPath}
}

// List returns all configured contexts sorted by name.
func (r *Registry) List() ([]NamedTarget, error) {
	doc, err := r.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(doc.Contexts))
	for name := range doc.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]NamedTarget, 0, len(names))
	for _, name := range names {
		result = append(result, NamedTarget{Name: name, Current: name == doc.Current, Target: doc.Contexts[name]})
	}
	return result, nil
}

// Current returns the persisted active context.
func (r *Registry) Current() (NamedTarget, error) {
	return r.Resolve("")
}

// Resolve returns an explicit context, or the persisted active context when
// name is empty.
func (r *Registry) Resolve(name string) (NamedTarget, error) {
	doc, err := r.load()
	if err != nil {
		return NamedTarget{}, err
	}
	if name == "" {
		name = doc.Current
	}
	target, ok := doc.Contexts[name]
	if !ok {
		return NamedTarget{}, fmt.Errorf("context %q does not exist", name)
	}
	return NamedTarget{Name: name, Current: name == doc.Current, Target: target}, nil
}

// Use persists name as the active context.
func (r *Registry) Use(name string) (NamedTarget, error) {
	doc, err := r.load()
	if err != nil {
		return NamedTarget{}, err
	}
	target, ok := doc.Contexts[name]
	if !ok {
		return NamedTarget{}, fmt.Errorf("context %q does not exist", name)
	}
	doc.Current = name
	if err := r.store.Write(doc); err != nil {
		return NamedTarget{}, err
	}
	return NamedTarget{Name: name, Current: true, Target: target}, nil
}

func (r *Registry) load() (Document, error) {
	doc, err := r.store.Load()
	if err != nil {
		return Document{}, err
	}
	if doc.Contexts == nil {
		doc.Contexts = map[string]Target{}
	}
	if _, exists := doc.Contexts[defaultContextName]; !exists {
		doc.Contexts[defaultContextName] = Target{Kind: KindLocal, ConfigPath: r.defaultLocalPath}
	}
	if doc.Current == "" {
		doc.Current = defaultContextName
	}
	for name, target := range doc.Contexts {
		if err := validate(name, target); err != nil {
			return Document{}, err
		}
	}
	if _, exists := doc.Contexts[doc.Current]; !exists {
		return Document{}, fmt.Errorf("current context %q does not exist", doc.Current)
	}
	return doc, nil
}

// Load reads the context document. A missing file is an empty registry rather
// than an error so Registry can supply the implicit local context.
func (s Store) Load() (Document, error) {
	doc := Document{Contexts: map[string]Target{}}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return Document{}, fmt.Errorf("read contexts: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("parse %s: %w", s.Path, err)
	}
	return doc, nil
}

// Write atomically replaces the context document with owner-only permissions.
func (s Store) Write(doc Document) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create contexts directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".contexts-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary contexts: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	encoder := yaml.NewEncoder(tmp)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return fmt.Errorf("encode contexts: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace contexts: %w", err)
	}
	committed = true
	return nil
}

func validate(name string, target Target) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("context name %q must match %s", name, namePattern)
	}
	switch target.Kind {
	case KindLocal:
		if strings.TrimSpace(target.ConfigPath) == "" {
			return fmt.Errorf("context %q: local config path is required", name)
		}
		if target.Endpoint != "" {
			return fmt.Errorf("context %q: local context cannot set endpoint", name)
		}
	case KindRemote:
		if err := validateEndpoint(target.Endpoint); err != nil {
			return fmt.Errorf("context %q: %w", name, err)
		}
		if target.ConfigPath != "" {
			return fmt.Errorf("context %q: remote context cannot set config", name)
		}
	default:
		return fmt.Errorf("context %q: kind must be local or remote", name)
	}
	return nil
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("remote endpoint must be an absolute HTTP(S) URL")
	}
	return nil
}
