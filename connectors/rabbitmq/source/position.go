package source

import (
	"fmt"
	"strconv"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

// PositionCodec names the next RabbitMQ Stream offset to consume.
const PositionCodec = "rabbitmq.stream.next_offset"

// OffsetCodec validates nonnegative RabbitMQ Stream offsets.
type OffsetCodec struct{}

func (OffsetCodec) Validate(p rowmodel.Position) error {
	if p.Codec != PositionCodec || p.Version != 0 {
		return fmt.Errorf("rabbitmq: incompatible offset codec")
	}
	if err := (streamkit.Uint64Codec{}).Validate(p); err != nil {
		return err
	}
	_, err := strconv.ParseInt(string(p.Value), 10, 64)
	return err
}

func (c OffsetCodec) Canonicalize(p rowmodel.Position) (rowmodel.Position, error) {
	if err := c.Validate(p); err != nil {
		return rowmodel.Position{}, err
	}
	return (streamkit.Uint64Codec{}).Canonicalize(p)
}

func (c OffsetCodec) Compare(a, b rowmodel.Position) (rowmodel.PositionOrder, error) {
	if a.Codec != b.Codec || a.Version != b.Version {
		return rowmodel.PositionIncomparable, nil
	}
	if err := c.Validate(a); err != nil {
		return rowmodel.PositionIncomparable, err
	}
	if err := c.Validate(b); err != nil {
		return rowmodel.PositionIncomparable, err
	}
	return (streamkit.Uint64Codec{}).Compare(a, b)
}

func (*Source) Lookup(name string, version int) (rowmodel.PositionCodec, error) {
	if name != PositionCodec || version != 0 {
		return nil, fmt.Errorf("%w: %s v%d", streamkit.ErrUnknownCodec, name, version)
	}
	return OffsetCodec{}, nil
}

func position(n int64) rowmodel.Position {
	return rowmodel.Position{Codec: PositionCodec, Version: 0, Value: []byte(strconv.FormatInt(n, 10))}
}
