// Package gzipencoder wraps an encoder in one streaming gzip member.
//
// Each batch's content is cut into blocks that are deflated concurrently and
// concatenated in order, the technique pgzip uses. A block's compressor is
// primed with the 32 KiB of content before it, the whole deflate window, so
// back-references across block boundaries still work and the ratio stays close
// to a serial stream. Every block ends with a sync flush, which leaves the
// stream byte-aligned so blocks join without bit shifting. The result is one
// ordinary gzip member that every reader decodes, unlike concatenated members,
// which some readers silently truncate after the first.
package gzipencoder

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"runtime"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/klauspost/compress/flate"

	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

const (
	// blockSize is the content compressed by one goroutine. Smaller blocks
	// spread a batch over more cores; each block costs a sync marker and a
	// fresh Huffman table.
	blockSize = 256 << 10
	// windowSize is the deflate back-reference window and so the dictionary
	// each block needs from the content before it.
	windowSize = 32 << 10
	level      = flate.DefaultCompression
)

// gzipHeader is the RFC 1952 member header the standard library writes for a
// writer with no name, comment, or modification time: deflate, no flags,
// mtime 0, no extra flags at the default level, unknown OS.
var gzipHeader = []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0xff}

// finalBlock is an empty fixed-Huffman deflate block with BFINAL set, the
// bytes a deflate writer emits when closed with nothing pending.
var finalBlock = []byte{0x03, 0x00}

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// slots bounds concurrent block compression across every encoder in the
// process, so many resources compressing at once cannot oversubscribe the CPU.
var slots = make(chan struct{}, runtime.GOMAXPROCS(0))

var writers = sync.Pool{New: func() any {
	w, err := flate.NewWriter(nil, level)
	if err != nil {
		panic(err) // level is a constant the package accepts
	}
	return w
}}

// Encoder incrementally compresses batches belonging to one Arrow schema.
//
// Compression of a batch runs in the background while the caller produces the
// next one, so JSON encoding and compression overlap instead of alternating.
// EncodeBatch therefore returns the compressed bytes of the previous batch, the
// way a Parquet encoder returns row groups later than their rows, and Finalize
// returns the last batch's bytes with the deflate end and the gzip trailer.
// The bytes are identical to compressing each batch synchronously.
type Encoder struct {
	json    encoder.Encoder
	raw     [2][]byte // JSON content, one buffer per batch in flight
	next    int       // raw buffer the next batch is encoded into
	window  []byte    // last windowSize bytes of content already compressed
	crc     uint32    // CRC-32 (IEEE) of all content, for the gzip trailer
	size    uint32    // content length modulo 2^32, for the gzip trailer
	started bool
	closed  bool
	blocks  []blockBuffer

	// The batch being compressed in the background, if any.
	inflight []byte
	done     chan struct{}
	out      []byte
	err      error
}

type blockBuffer struct{ out []byte }

func (b *blockBuffer) Write(p []byte) (int, error) {
	b.out = append(b.out, p...)
	return len(p), nil
}

// NewEncoder prepares a gzip stream around a JSON encoder.
func NewEncoder(jsonEncoder encoder.Encoder) *Encoder {
	return &Encoder{json: jsonEncoder, window: make([]byte, 0, windowSize)}
}

// EncodeBatch encodes rows and starts compressing them in the background. It
// returns the compressed bytes of the previous batch, which can be none, plus
// their CRC32C.
func (e *Encoder) EncodeBatch(dst []byte, rows arrow.RecordBatch) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("gzip: encode after finalize")
	}
	// The previous batch is still compressing from the other raw buffer.
	content, _, err := e.json.EncodeBatch(e.raw[e.next][:0], rows)
	if err != nil {
		return nil, 0, fmt.Errorf("gzip: encode content: %w", err)
	}
	e.raw[e.next] = content
	start := len(dst)
	dst = e.header(dst)
	if dst, err = e.collect(dst); err != nil {
		return nil, 0, err
	}
	e.launch(content)
	e.next ^= 1
	return dst, crc32.Checksum(dst[start:], castagnoli), nil
}

// Finalize closes the gzip stream and returns its last bytes.
func (e *Encoder) Finalize(dst []byte) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("gzip: already finalized")
	}
	e.closed = true
	content, _, err := e.json.Finalize(e.raw[e.next][:0])
	if err != nil {
		return nil, 0, fmt.Errorf("gzip: finalize content: %w", err)
	}
	e.raw[e.next] = content
	start := len(dst)
	dst = e.header(dst)
	if dst, err = e.collect(dst); err != nil {
		return nil, 0, err
	}
	if dst, err = e.compressBlocks(dst, e.window, content); err != nil {
		return nil, 0, err
	}
	e.advance(content)
	dst = append(dst, finalBlock...)
	dst = binary.LittleEndian.AppendUint32(dst, e.crc)
	dst = binary.LittleEndian.AppendUint32(dst, e.size)
	return dst, crc32.Checksum(dst[start:], castagnoli), nil
}

func (e *Encoder) header(dst []byte) []byte {
	if e.started {
		return dst
	}
	e.started = true
	return append(dst, gzipHeader...)
}

// launch starts compressing content in the background. The previous batch
// must have been collected, so the window already ends with its content.
func (e *Encoder) launch(content []byte) {
	if len(content) == 0 {
		return
	}
	e.inflight = content
	e.done = make(chan struct{})
	dict := e.window
	go func() {
		defer close(e.done)
		e.out, e.err = e.compressBlocks(e.out[:0], dict, content)
	}()
}

// collect waits for the background batch, appends its bytes to dst, and
// advances the trailer checksum and the window past its content.
func (e *Encoder) collect(dst []byte) ([]byte, error) {
	if e.inflight == nil {
		return dst, nil
	}
	<-e.done
	content := e.inflight
	e.inflight = nil
	if e.err != nil {
		return nil, e.err
	}
	e.advance(content)
	return append(dst, e.out...), nil
}

func (e *Encoder) advance(content []byte) {
	e.crc = crc32.Update(e.crc, crc32.IEEETable, content)
	e.size += uint32(len(content)) //nolint:gosec // ISIZE is defined modulo 2^32
	e.window = appendWindow(e.window, content)
}

// compressBlocks appends content as sync-flushed deflate blocks, compressing
// the blocks concurrently. The first block is primed with window, the content
// that precedes this one in the stream.
func (e *Encoder) compressBlocks(dst, window, content []byte) ([]byte, error) {
	if len(content) == 0 {
		return dst, nil
	}
	count := (len(content) + blockSize - 1) / blockSize
	if cap(e.blocks) < count {
		e.blocks = make([]blockBuffer, count)
	}
	blocks := e.blocks[:count]
	errs := make([]error, count)

	var wg sync.WaitGroup
	for i := range blocks {
		from, to := i*blockSize, min((i+1)*blockSize, len(content))
		dict := window
		if i > 0 {
			dict = content[max(0, from-windowSize):from]
		}
		block := &blocks[i]
		block.out = block.out[:0]
		if count == 1 {
			errs[i] = deflateBlock(block, dict, content[from:to])
			break
		}
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			errs[i] = deflateBlock(block, dict, content[from:to])
		})
	}
	wg.Wait()
	for i := range blocks {
		if errs[i] != nil {
			return nil, fmt.Errorf("gzip: compress block: %w", errs[i])
		}
		dst = append(dst, blocks[i].out...)
	}
	return dst, nil
}

func deflateBlock(out *blockBuffer, dict, content []byte) error {
	w := writers.Get().(*flate.Writer)
	defer writers.Put(w)
	w.ResetDict(out, dict)
	if _, err := w.Write(content); err != nil {
		return err
	}
	return w.Flush()
}

// appendWindow keeps the last windowSize bytes of window followed by content.
func appendWindow(window, content []byte) []byte {
	if len(content) >= windowSize {
		return append(window[:0], content[len(content)-windowSize:]...)
	}
	if keep := windowSize - len(content); len(window) > keep {
		window = append(window[:0], window[len(window)-keep:]...)
	}
	return append(window, content...)
}
