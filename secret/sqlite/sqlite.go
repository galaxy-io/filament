// Package sqlite implements filament.Secrets over the secrets table in the
// shared Filament SQLite schema, mirroring secret/postgres.
package sqlite

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// Provider encrypts secret values with AES-GCM before persisting them.
type Provider struct {
	q     *sqlcgen.Queries
	keyID string
	gcm   cipher.AEAD
}

var _ filament.Secrets = (*Provider)(nil)

// NewFromEnv constructs a provider from ENCRYPTION_KEY (base64-encoded AES
// key) and ENCRYPTION_KEY_ID (defaults to "default").
func NewFromEnv(db *sql.DB) (*Provider, error) {
	encoded := os.Getenv("ENCRYPTION_KEY")
	if encoded == "" {
		return nil, errors.New("secret/sqlite: ENCRYPTION_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("secret/sqlite: ENCRYPTION_KEY must be base64: %w", err)
	}
	keyID := os.Getenv("ENCRYPTION_KEY_ID")
	if keyID == "" {
		keyID = "default"
	}
	return New(db, keyID, key)
}

// New constructs a provider over the handle shared with datastore/sqlite.Store
// (see its DB method). key must be 16, 24, or 32 bytes.
func New(db *sql.DB, keyID string, key []byte) (*Provider, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret/sqlite: key: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret/sqlite: gcm: %w", err)
	}
	return &Provider{q: sqlcgen.New(db), keyID: keyID, gcm: gcm}, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string { return "sqlite" }

func (p *Provider) Write(ctx context.Context, ref string, secret filament.Secret) error {
	if secret.Tenant == "" {
		return fmt.Errorf("secret/sqlite: tenant is required")
	}
	nonce := make([]byte, p.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("secret/sqlite: nonce: %w", err)
	}
	meta, err := json.Marshal(secret.Meta)
	if err != nil {
		return fmt.Errorf("secret/sqlite: marshal metadata: %w", err)
	}
	now := time.Now().UnixMilli()
	err = p.q.WriteSecret(ctx, sqlcgen.WriteSecretParams{
		TenantID: string(secret.Tenant), Ref: ref,
		Ciphertext: p.gcm.Seal(nil, nonce, secret.Value, nil), Nonce: nonce,
		KeyID: p.keyID, Metadata: string(meta), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("secret/sqlite: write: %w", err)
	}
	return nil
}

func (p *Provider) Read(ctx context.Context, ref string) (filament.Secret, error) {
	row, err := p.q.ReadSecret(ctx, ref)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return filament.Secret{}, fmt.Errorf("read secret %q: %w", ref, filament.ErrNotFound)
		}
		return filament.Secret{}, fmt.Errorf("secret/sqlite: read: %w", err)
	}
	value, err := p.gcm.Open(nil, row.Nonce, row.Ciphertext, nil)
	if err != nil {
		return filament.Secret{}, fmt.Errorf("secret/sqlite: decrypt %q: %w", ref, err)
	}
	var meta map[string]string
	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &meta); err != nil {
			return filament.Secret{}, fmt.Errorf("secret/sqlite: unmarshal metadata: %w", err)
		}
	}
	return filament.Secret{Value: value, Meta: meta}, nil
}

// Delete removes the secret at ref; deleting a missing ref is a no-op.
func (p *Provider) Delete(ctx context.Context, ref string) error {
	if err := p.q.DeleteSecret(ctx, ref); err != nil {
		return fmt.Errorf("secret/sqlite: delete: %w", err)
	}
	return nil
}
