package secret

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/secret/env"
	"github.com/galaxy-io/filament/secret/sqlite"
)

// Local stores secrets in the sqlite datastore and reads references it does
// not hold from the environment, so a single-process deployment accepts
// plaintext values and env references alike. The key is ENCRYPTION_KEY when
// set, otherwise the key at keyPath, minted on first use. Losing that file
// loses the stored secrets, not the rest of the store.
func Local(db *sql.DB, keyPath string) (filament.Secrets, error) {
	if os.Getenv("ENCRYPTION_KEY") != "" {
		stored, err := sqlite.NewFromEnv(db)
		if err != nil {
			return nil, err
		}
		return localSecrets{stored: stored, env: env.New()}, nil
	}
	key, err := localKey(keyPath)
	if err != nil {
		return nil, err
	}
	stored, err := sqlite.New(db, "local", key)
	if err != nil {
		return nil, err
	}
	return localSecrets{stored: stored, env: env.New()}, nil
}

func localKey(path string) ([]byte, error) {
	encoded, err := os.ReadFile(path)
	if err == nil {
		return base64.StdEncoding.DecodeString(string(encoded))
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

type localSecrets struct {
	stored *sqlite.Provider
	env    filament.Secrets
}

func (s localSecrets) Name() string { return "local" }

// RotateSecret delegates refresh-token writes to the sqlite layer.
func (s localSecrets) RotateSecret(ctx context.Context, ref string, secret filament.Secret) error {
	return s.stored.RotateSecret(ctx, ref, secret)
}

func (s localSecrets) Read(ctx context.Context, ref string) (filament.Secret, error) {
	secret, err := s.stored.Read(ctx, ref)
	if errors.Is(err, filament.ErrNotFound) {
		return s.env.Read(ctx, ref)
	}
	return secret, err
}

func (s localSecrets) Write(ctx context.Context, ref string, secret filament.Secret) error {
	return s.stored.Write(ctx, ref, secret)
}

func (s localSecrets) Delete(ctx context.Context, ref string) error {
	return s.stored.Delete(ctx, ref)
}
