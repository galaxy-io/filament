// Package auth stores service-account credentials and mints the access
// tokens remote targets attach to their requests.
package auth

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Profile holds one service account's credentials and where to redeem them.
// Issuer and Scopes are captured from the server at login so minting a token
// never needs an unauthenticated round trip first.
type Profile struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes,omitempty"`
	Cache        *Cache   `yaml:"cache,omitempty"`
}

// Cache is a previously minted access token retained until expiry.
type Cache struct {
	AccessToken string    `yaml:"access_token"`
	ExpiresAt   time.Time `yaml:"expires_at"`
}

// Document is the persisted credential file, keyed by auth profile name.
// Context registries reference profiles by name and never hold secrets.
type Document struct {
	Profiles map[string]Profile `yaml:"profiles"`
}

// Store atomically persists credentials with owner-only permissions.
type Store struct {
	Path string
}

// Get returns a stored profile.
func (s Store) Get(name string) (Profile, error) {
	doc, err := s.Load()
	if err != nil {
		return Profile{}, err
	}
	profile, ok := doc.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("auth profile %q does not exist", name)
	}
	return profile, nil
}

// Put validates and stores a profile, replacing any previous credentials and
// cached token under the same name.
func (s Store) Put(name string, profile Profile) error {
	if name == "" {
		return errors.New("auth profile name is required")
	}
	if profile.Issuer == "" || profile.ClientID == "" || profile.ClientSecret == "" {
		return fmt.Errorf("auth profile %q needs issuer, client id, and client secret", name)
	}
	doc, err := s.Load()
	if err != nil {
		return err
	}
	doc.Profiles[name] = profile
	return s.Write(doc)
}

// Delete removes a profile. Deleting an absent profile is not an error.
func (s Store) Delete(name string) error {
	doc, err := s.Load()
	if err != nil {
		return err
	}
	if _, ok := doc.Profiles[name]; !ok {
		return nil
	}
	delete(doc.Profiles, name)
	return s.Write(doc)
}

// SaveCache persists a freshly minted token against an existing profile.
func (s Store) SaveCache(name string, cache Cache) error {
	doc, err := s.Load()
	if err != nil {
		return err
	}
	profile, ok := doc.Profiles[name]
	if !ok {
		return fmt.Errorf("auth profile %q does not exist", name)
	}
	profile.Cache = &cache
	doc.Profiles[name] = profile
	return s.Write(doc)
}

// Load reads the credential document. A missing file is an empty store
// rather than an error.
func (s Store) Load() (Document, error) {
	doc := Document{Profiles: map[string]Profile{}}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return Document{}, fmt.Errorf("read credentials: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("parse %s: %w", s.Path, err)
	}
	if doc.Profiles == nil {
		doc.Profiles = map[string]Profile{}
	}
	return doc, nil
}

// Write atomically replaces the credential document with owner-only
// permissions.
func (s Store) Write(doc Document) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".credentials-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary credentials: %w", err)
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
		return fmt.Errorf("encode credentials: %w", err)
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
		return fmt.Errorf("replace credentials: %w", err)
	}
	committed = true
	return nil
}
