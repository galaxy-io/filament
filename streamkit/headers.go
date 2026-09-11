package streamkit

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var ErrInvalidEnvelope = errors.New("streamkit: invalid envelope")

// Header preserves provider order, duplicate names and arbitrary binary values.
// Null is authoritative; a null header with nonempty bytes is invalid.
type Header struct {
	Key   string
	Value []byte
	Null  bool
}

// EncodeHeaders v1: version byte, presence byte, u32 count; each entry is a
// u32-length key, null byte, u32-length value. Integers are big endian. Nil and
// empty lists differ; null values differ from non-null empty bytes.
func EncodeHeaders(headers []Header) ([]byte, error) {
	out := []byte{1, 0}
	if headers == nil {
		return out, nil
	}
	out[1] = 1
	if uint64(len(headers)) > uint64(^uint32(0)) {
		return nil, ErrInvalidEnvelope
	}
	out = binary.BigEndian.AppendUint32(out, uint32(len(headers)))
	for _, h := range headers {
		if h.Null && len(h.Value) > 0 {
			return nil, ErrInvalidEnvelope
		}
		if uint64(len(h.Key)) > uint64(^uint32(0)) || uint64(len(h.Value)) > uint64(^uint32(0)) {
			return nil, ErrInvalidEnvelope
		}
		out = binary.BigEndian.AppendUint32(out, uint32(len(h.Key)))
		out = append(out, h.Key...)
		flag := byte(0)
		if h.Null {
			flag = 1
		}
		out = append(out, flag)
		out = binary.BigEndian.AppendUint32(out, uint32(len(h.Value)))
		out = append(out, h.Value...)
	}
	return out, nil
}
func DecodeHeaders(data []byte) ([]Header, error) {
	if len(data) < 2 || data[0] != 1 || data[1] > 1 {
		return nil, ErrInvalidEnvelope
	}
	if data[1] == 0 {
		if len(data) != 2 {
			return nil, ErrInvalidEnvelope
		}
		return nil, nil
	}
	r := bytes.NewReader(data[2:])
	var count uint32
	if binary.Read(r, binary.BigEndian, &count) != nil || uint64(count) > uint64(r.Len()/9) {
		return nil, ErrInvalidEnvelope
	}
	readBytes := func() ([]byte, error) {
		var n uint32
		if binary.Read(r, binary.BigEndian, &n) != nil || uint64(n) > uint64(r.Len()) {
			return nil, ErrInvalidEnvelope
		}
		b := make([]byte, int(n))
		_, err := io.ReadFull(r, b)
		return b, err
	}
	out := make([]Header, 0, int(count))
	for range count {
		key, err := readBytes()
		if err != nil {
			return nil, ErrInvalidEnvelope
		}
		flag, err := r.ReadByte()
		if err != nil || flag > 1 {
			return nil, ErrInvalidEnvelope
		}
		value, err := readBytes()
		if err != nil || (flag == 1 && len(value) > 0) {
			return nil, ErrInvalidEnvelope
		}
		if flag == 1 {
			value = nil
		}
		out = append(out, Header{string(key), value, flag == 1})
	}
	if r.Len() != 0 {
		return nil, ErrInvalidEnvelope
	}
	return out, nil
}
