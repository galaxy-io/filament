// Package postgres implements ingestion.Secrets using the shared Filament Postgres schema.
package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// Provider encrypts secret values with AES-GCM before persisting them.
type Provider struct {
	q     *sqlcgen.Queries
	keyID string
	gcm   cipher.AEAD
}

var _ ingestion.Secrets = (*Provider)(nil)

// New constructs a provider. key must be 16, 24, or 32 bytes.
func New(pool *pgxpool.Pool, keyID string, key []byte) (*Provider, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret/postgres: key: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret/postgres: gcm: %w", err)
	}
	return &Provider{q: sqlcgen.New(pool), keyID: keyID, gcm: gcm}, nil
}
func (p *Provider) Name() string { return "postgres" }
func (p *Provider) Write(ctx context.Context, ref string, secret ingestion.Secret) error {
	nonce := make([]byte, p.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("secret/postgres: nonce: %w", err)
	}
	meta, err := json.Marshal(secret.Meta)
	if err != nil {
		return fmt.Errorf("secret/postgres: marshal metadata: %w", err)
	}
	err = p.q.WriteSecret(ctx, sqlcgen.WriteSecretParams{Ref: ref, Ciphertext: p.gcm.Seal(nil, nonce, secret.Value, nil), Nonce: nonce, KeyID: p.keyID, Metadata: meta})
	if err != nil {
		return fmt.Errorf("secret/postgres: write: %w", err)
	}
	return nil
}

func (p *Provider) Read(ctx context.Context, ref string) (ingestion.Secret, error) {
	row, err := p.q.ReadSecret(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ingestion.Secret{}, fmt.Errorf("read secret %q: %w", ref, ingestion.ErrNotFound)
		}
		return ingestion.Secret{}, fmt.Errorf("secret/postgres: read: %w", err)
	}
	value, err := p.gcm.Open(nil, row.Nonce, row.Ciphertext, nil)
	if err != nil {
		return ingestion.Secret{}, fmt.Errorf("secret/postgres: decrypt %q: %w", ref, err)
	}
	var meta map[string]string
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &meta); err != nil {
			return ingestion.Secret{}, fmt.Errorf("secret/postgres: unmarshal metadata: %w", err)
		}
	}
	return ingestion.Secret{Value: value, Meta: meta}, nil
}

func (p *Provider) Delete(ctx context.Context, ref string) error {
	if err := p.q.DeleteSecret(ctx, ref); err != nil {
		return fmt.Errorf("secret/postgres: delete: %w", err)
	}
	return nil
}
