package gzipencoder

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
)

// chunkEncoder stands in for the JSON encoder: each EncodeBatch emits the next
// chunk and Finalize emits a fixed trailer, so tests control block boundaries.
type chunkEncoder struct {
	chunks  [][]byte
	trailer []byte
}

func (c *chunkEncoder) EncodeBatch(dst []byte, _ arrow.RecordBatch) ([]byte, uint32, error) {
	next := c.chunks[0]
	c.chunks = c.chunks[1:]
	return append(dst, next...), 0, nil
}

func (c *chunkEncoder) Finalize(dst []byte) ([]byte, uint32, error) {
	return append(dst, c.trailer...), 0, nil
}

// jsonLike returns n bytes of repetitive but varied NDJSON-shaped text.
func jsonLike(seed uint64, n int) []byte {
	random := rand.New(rand.NewPCG(seed, 1))
	words := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	var out bytes.Buffer
	for out.Len() < n {
		fmt.Fprintf(&out, `{"id":%d,"account":"acct-%d","note":"`, random.Int64(), random.IntN(50_000))
		for range 4 + random.IntN(20) {
			out.WriteString(words[random.IntN(len(words))])
			out.WriteByte(' ')
		}
		out.WriteString("\"}\n")
	}
	return out.Bytes()[:n]
}

func encodeAll(t *testing.T, chunks [][]byte, trailer []byte) []byte {
	t.Helper()
	enc := NewEncoder(&chunkEncoder{chunks: append([][]byte(nil), chunks...), trailer: trailer})
	var out []byte
	for range chunks {
		part, checksum, err := enc.EncodeBatch(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := crc32.Checksum(part, castagnoli); checksum != want {
			t.Fatalf("batch CRC = %08x, want %08x", checksum, want)
		}
		out = append(out, part...)
	}
	tail, checksum, err := enc.Finalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := crc32.Checksum(tail, castagnoli); checksum != want {
		t.Fatalf("tail CRC = %08x, want %08x", checksum, want)
	}
	return append(out, tail...)
}

// decodeOneMember decodes with the standard library, independent of the
// compressor, and fails if the stream is not exactly one gzip member.
func decodeOneMember(t *testing.T, compressed []byte) []byte {
	t.Helper()
	input := bytes.NewReader(compressed)
	reader, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	reader.Multistream(false)
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if input.Len() != 0 {
		t.Fatalf("%d bytes after the first gzip member", input.Len())
	}
	return decoded
}

func TestParallelStreamIsOneMemberWithExactContent(t *testing.T) {
	for _, test := range []struct {
		name  string
		sizes []int
	}{
		{"tiny batches", []int{1, 7, 0, 300}},
		{"exact block", []int{blockSize}},
		{"window boundaries", []int{windowSize - 1, 1, windowSize + 1, 3}},
		{"many blocks", []int{5*blockSize + 123, 2*blockSize - 1}},
		{"mixed", []int{100, 3 * blockSize, 10, blockSize + 1, windowSize / 2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var chunks [][]byte
			var want []byte
			for i, size := range test.sizes {
				chunk := jsonLike(uint64(i), size)
				chunks = append(chunks, chunk)
				want = append(want, chunk...)
			}
			trailer := []byte("]\n")
			want = append(want, trailer...)

			compressed := encodeAll(t, chunks, trailer)
			if got := decodeOneMember(t, compressed); !bytes.Equal(got, want) {
				t.Fatalf("decoded %d bytes, want %d; content differs", len(got), len(want))
			}
			if again := encodeAll(t, chunks, trailer); !bytes.Equal(again, compressed) {
				t.Fatal("output depends on scheduling")
			}
		})
	}
}

func TestParallelStreamEmpty(t *testing.T) {
	compressed := encodeAll(t, nil, nil)
	if got := decodeOneMember(t, compressed); len(got) != 0 {
		t.Fatalf("empty stream decoded to %q", got)
	}
}
