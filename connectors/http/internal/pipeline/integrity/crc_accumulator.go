package integrity

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

// castagnoliTable uses the Castagnoli polynomial, which has hardware
// acceleration on both ARM64 (CRC32 instructions) and x86 (SSE4.2).
var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

// Accumulator tracks a running CRC32C over ingested records.
// Each record is fed into a running CRC that includes a length prefix
// to prevent ambiguity between records.
type Accumulator struct {
	RecordCount int64
	ByteCount   int64
	CRC         uint32
}

// NewAccumulator creates an Accumulator.
func NewAccumulator() *Accumulator {
	return &Accumulator{}
}

// Ingest adds a record's bytes to the running CRC.
// Feeds a little-endian length prefix before the data to ensure
// that two records ["ab","c"] and ["a","bc"] produce different CRCs.
func (a *Accumulator) Ingest(data []byte) {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	a.CRC = crc32.Update(a.CRC, castagnoliTable, lenBuf[:])
	a.CRC = crc32.Update(a.CRC, castagnoliTable, data)
	a.RecordCount++
	a.ByteCount += int64(len(data))
}

// Commitment is the accumulator's snapshot for comparison and serialization.
type Commitment struct {
	RecordCount int64  `json:"record_count"`
	ByteCount   int64  `json:"byte_count"`
	CRC         uint32 `json:"crc"`
}

func (a *Accumulator) Commitment() Commitment {
	return Commitment{
		RecordCount: a.RecordCount,
		ByteCount:   a.ByteCount,
		CRC:         a.CRC,
	}
}

// Equal returns true if two commitments are identical.
func (c Commitment) Equal(other Commitment) bool {
	return c.RecordCount == other.RecordCount &&
		c.ByteCount == other.ByteCount &&
		c.CRC == other.CRC
}

// CanonicalBytes returns a deterministic 20-byte encoding.
// Layout: RecordCount(8) | ByteCount(8) | CRC(4), all little-endian.
func (c Commitment) CanonicalBytes() [20]byte {
	var buf [20]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(c.RecordCount))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(c.ByteCount))
	binary.LittleEndian.PutUint32(buf[16:20], c.CRC)
	return buf
}

// ChunkCommitmentResult pairs a write-side commitment with its sequence number
// so that callers can assert read/write sequence alignment.
type ChunkCommitmentResult struct {
	Commitment Commitment
	SeqNum     int
}

// Verify reads an NDJSON stream line by line and checks against a commitment.
func Verify(r io.Reader, expected Commitment) error {
	actual, err := ComputeCommitment(r)
	if err != nil {
		return err
	}
	if actual.RecordCount != expected.RecordCount {
		return fmt.Errorf("record count mismatch: expected=%d actual=%d", expected.RecordCount, actual.RecordCount)
	}
	if actual.ByteCount != expected.ByteCount {
		return fmt.Errorf("byte count mismatch: expected=%d actual=%d", expected.ByteCount, actual.ByteCount)
	}
	if actual.CRC != expected.CRC {
		return fmt.Errorf("CRC mismatch: expected=%08x actual=%08x", expected.CRC, actual.CRC)
	}
	return nil
}

// ComputeAccumulator reads an NDJSON stream and returns a populated Accumulator.
func ComputeAccumulator(r io.Reader) (*Accumulator, error) {
	acc := NewAccumulator()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		acc.Ingest(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return acc, nil
}

// ComputeCommitment reads an NDJSON stream and returns its CRC commitment.
func ComputeCommitment(r io.Reader) (Commitment, error) {
	acc := NewAccumulator()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		acc.Ingest(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return Commitment{}, fmt.Errorf("scan: %w", err)
	}
	return acc.Commitment(), nil
}
