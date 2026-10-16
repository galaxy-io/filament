package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// UpdateSecret runs one read-modify-write under SQLite's writer lock. Acquire
// that lock before reading so independent processes cannot rotate the same
// token concurrently. The callback must use ctx and must not reenter the DB.
func (p *Provider) UpdateSecret(ctx context.Context, tenant filament.TenantID, ref string, fn func(filament.Secret) (filament.Secret, error)) (filament.Secret, error) {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return filament.Secret{}, err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return filament.Secret{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "ROLLBACK")
	}()
	q := sqlcgen.New(conn)
	row, err := q.LockSecret(ctx, sqlcgen.LockSecretParams{TenantID: string(tenant), Ref: ref})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = filament.ErrNotFound
		}
		return filament.Secret{}, err
	}
	bound := *p
	bound.q = q
	current, err := p.decodeSecret(ref, row.Ciphertext, row.Nonce, []byte(row.Metadata))
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
		return filament.Secret{}, fmt.Errorf("secret/sqlite: cannot change secret tenant")
	}
	if !bytes.Equal(next.Value, current.Value) || !maps.Equal(next.Meta, current.Meta) {
		if err := bound.Write(ctx, ref, next); err != nil {
			return filament.Secret{}, err
		}
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return filament.Secret{}, err
	}
	return next, nil
}
