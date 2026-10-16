package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// UpdateSecret serializes read-modify-write against refreshes, ordinary writes,
// and deletes. The callback runs once under the row lock and must use ctx for
// network work. A missing secret is never recreated.
func (p *Provider) UpdateSecret(ctx context.Context, tenant filament.TenantID, ref string, fn func(filament.Secret) (filament.Secret, error)) (filament.Secret, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return filament.Secret{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	q := p.q.WithTx(tx)
	row, err := q.LockSecret(ctx, sqlcgen.LockSecretParams{TenantID: string(tenant), Ref: ref})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = filament.ErrNotFound
		}
		return filament.Secret{}, err
	}
	bound := *p
	bound.q = q
	current, err := p.decodeSecret(ref, row.Ciphertext, row.Nonce, row.Metadata)
	if err != nil {
		return filament.Secret{}, err
	}
	current.Tenant = tenant
	input := current
	input.Value, input.Meta = bytes.Clone(current.Value), maps.Clone(current.Meta)
	next, err := fn(input)
	if err != nil {
		return filament.Secret{}, err
	}
	if next.Tenant != tenant {
		return filament.Secret{}, fmt.Errorf("secret/postgres: cannot change secret tenant")
	}
	if !bytes.Equal(next.Value, current.Value) || !maps.Equal(next.Meta, current.Meta) {
		if err := bound.Write(ctx, ref, next); err != nil {
			return filament.Secret{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.Secret{}, err
	}
	return next, nil
}
