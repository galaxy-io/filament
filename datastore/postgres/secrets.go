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

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// SecretsStore is a Postgres-backed ingestion.Secrets implementation.
type SecretsStore struct {
	q     *sqlcgen.Queries
	keyID string
	gcm   cipher.AEAD
}

var _ ingestion.Secrets = (*SecretsStore)(nil)

// NewSecretsStore builds a SecretsStore. key must be 16, 24, or 32 bytes
// (AES-128/192/256); keyID is stamped on every row so keys can be rotated
// later by re-encrypting rows whose key_id != the current key.
func NewSecretsStore(pool *pgxpool.Pool, keyID string, key []byte) (*SecretsStore, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: secrets key: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: secrets gcm: %w", err)
	}
	return &SecretsStore{q: sqlcgen.New(pool), keyID: keyID, gcm: gcm}, nil
}

func (s *SecretsStore) Name() string { return "postgres" }

func (s *SecretsStore) Write(ctx context.Context, ref string, secret ingestion.Secret) error {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("datastore/postgres: nonce: %w", err)
	}
	ciphertext := s.gcm.Seal(nil, nonce, secret.Value, nil)

	meta, err := json.Marshal(secret.Meta)
	if err != nil {
		return fmt.Errorf("datastore/postgres: marshal secret meta: %w", err)
	}

	err = s.q.WriteSecret(ctx, sqlcgen.WriteSecretParams{
		Ref:        ref,
		Ciphertext: ciphertext,
		Nonce:      nonce,
		KeyID:      s.keyID,
		Metadata:   meta,
	})
	if err != nil {
		return fmt.Errorf("datastore/postgres: write secret: %w", err)
	}
	return nil
}

func (s *SecretsStore) Read(ctx context.Context, ref string) (ingestion.Secret, error) {
	row, err := s.q.ReadSecret(ctx, ref)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ingestion.Secret{}, fmt.Errorf("read secret %q: %w", ref, ingestion.ErrNotFound)
		}
		return ingestion.Secret{}, fmt.Errorf("datastore/postgres: read secret: %w", err)
	}

	value, err := s.gcm.Open(nil, row.Nonce, row.Ciphertext, nil)
	if err != nil {
		return ingestion.Secret{}, fmt.Errorf("datastore/postgres: decrypt secret %q: %w", ref, err)
	}
	var metaMap map[string]string
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &metaMap); err != nil {
			return ingestion.Secret{}, fmt.Errorf("datastore/postgres: unmarshal secret meta: %w", err)
		}
	}
	return ingestion.Secret{Value: value, Meta: metaMap}, nil
}

func (s *SecretsStore) Delete(ctx context.Context, ref string) error {
	if err := s.q.DeleteSecret(ctx, ref); err != nil {
		return fmt.Errorf("datastore/postgres: delete secret: %w", err)
	}
	return nil
}
