package streamkit

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// ErrInvalidEnvelope reports malformed or contradictory envelope data.
var ErrInvalidEnvelope = errors.New("streamkit: invalid envelope")

// HeaderEncodingVersion identifies the ordered binary header encoding.
const HeaderEncodingVersion byte = 1

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
	out := []byte{HeaderEncodingVersion, 0}
	if headers == nil {
		return out, nil
	}
	out[1] = 1
	count, err := headerLength(len(headers))
	if err != nil {
		return nil, ErrInvalidEnvelope
	}
	out = binary.BigEndian.AppendUint32(out, count)
	for _, h := range headers {
		if h.Null && len(h.Value) > 0 {
			return nil, ErrInvalidEnvelope
		}
		keySize, err := headerLength(len(h.Key))
		if err != nil {
			return nil, err
		}
		valueSize, err := headerLength(len(h.Value))
		if err != nil {
			return nil, ErrInvalidEnvelope
		}
		out = binary.BigEndian.AppendUint32(out, keySize)
		out = append(out, h.Key...)
		flag := byte(0)
		if h.Null {
			flag = 1
		}
		out = append(out, flag)
		out = binary.BigEndian.AppendUint32(out, valueSize)
		out = append(out, h.Value...)
	}
	return out, nil
}

// DecodeHeaders validates and decodes headers into independently owned values.
func DecodeHeaders(data []byte) ([]Header, error) {
	if len(data) < 2 || data[0] != HeaderEncodingVersion || data[1] > 1 {
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
	if binary.Read(r, binary.BigEndian, &count) != nil || int64(count) > int64(r.Len()/9) {
		return nil, ErrInvalidEnvelope
	}
	readBytes := func() ([]byte, error) {
		var n uint32
		if binary.Read(r, binary.BigEndian, &n) != nil || int64(n) > int64(r.Len()) {
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

// headerLength checks the wire limit before narrowing a platform-sized length.
func headerLength(n int) (uint32, error) {
	size := int64(n)
	if size < 0 || size > math.MaxUint32 {
		return 0, ErrInvalidEnvelope
	}
	return uint32(size), nil
}
