package postgres

import (
	"fmt"

	"github.com/jackc/pglogrepl"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/streamkit"
)

// PositionCodec identifies PostgreSQL transaction-end WAL positions.
const PositionCodec = "postgres.wal.lsn"

// LSNCodec compares WAL addresses numerically and canonicalizes PostgreSQL hex notation.
type LSNCodec struct{}

// Validate checks the codec version and PostgreSQL LSN syntax.
func (LSNCodec) Validate(p filament.Position) error {
	if p.Codec != PositionCodec || p.Version != 0 {
		return fmt.Errorf("postgres: unsupported WAL position codec")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := pglogrepl.ParseLSN(string(p.Value))
	return err
}

// Canonicalize returns the standard hexadecimal LSN representation.
func (c LSNCodec) Canonicalize(p filament.Position) (filament.Position, error) {
	if err := c.Validate(p); err != nil {
		return filament.Position{}, err
	}
	lsn, _ := pglogrepl.ParseLSN(string(p.Value))
	return walPosition(lsn), nil
}

// Compare orders positions numerically within the WAL domain.
func (c LSNCodec) Compare(a, b filament.Position) (filament.PositionOrder, error) {
	if a.Codec != b.Codec || a.Version != b.Version {
		return filament.PositionIncomparable, nil
	}
	if err := c.Validate(a); err != nil {
		return filament.PositionIncomparable, err
	}
	if err := c.Validate(b); err != nil {
		return filament.PositionIncomparable, err
	}
	x, _ := pglogrepl.ParseLSN(string(a.Value))
	y, _ := pglogrepl.ParseLSN(string(b.Value))
	if x < y {
		return filament.PositionBefore, nil
	}
	if x > y {
		return filament.PositionAfter, nil
	}
	return filament.PositionEqual, nil
}

// Lookup resolves the PostgreSQL WAL codec without opening a connection.
func (*Source) Lookup(name string, version int) (filament.PositionCodec, error) {
	if name != PositionCodec || version != 0 {
		return nil, fmt.Errorf("%w: %s v%d", streamkit.ErrUnknownCodec, name, version)
	}
	return LSNCodec{}, nil
}

func walPosition(lsn pglogrepl.LSN) filament.Position {
	return filament.Position{Codec: PositionCodec, Value: []byte(lsn.String())}
}
