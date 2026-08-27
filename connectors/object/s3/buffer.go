package s3

import (
	"errors"
	"io"
	"sync"
)

const (
	bufferChunkSize   = 64 << 10
	minFirstChunkSize = 4 << 10
)

// bufferPool keeps the common full-sized chunks reusable while allowing the
// first chunk of a small object to remain proportional to the object itself.
type bufferPool struct{ chunks sync.Pool }

func newBufferPool() *bufferPool {
	p := &bufferPool{}
	p.chunks.New = func() any {
		chunk := make([]byte, bufferChunkSize)
		return &chunk
	}
	return p
}

func (p *bufferPool) take(size int, small bool) []byte {
	if small && size < bufferChunkSize {
		capacity := minFirstChunkSize
		for capacity < size {
			capacity <<= 1
		}
		return make([]byte, capacity)
	}
	return *p.chunks.Get().(*[]byte)
}

func (p *bufferPool) put(chunk []byte) {
	if cap(chunk) == bufferChunkSize {
		chunk = chunk[:bufferChunkSize]
		p.chunks.Put(&chunk)
	}
}

// partBuffer is an append-only, bounded, seekable multipart request body. It
// avoids reserving partSize bytes for every active resource and remains
// seekable so the AWS SDK can retry requests safely.
type partBuffer struct {
	pool       *bufferPool
	limit      int
	chunks     [][]byte
	size       int
	position   int64
	readChunk  int
	readOffset int
	released   bool
}

func newPartBuffer(pool *bufferPool, limit int) *partBuffer {
	return &partBuffer{pool: pool, limit: limit}
}

func (b *partBuffer) Len() int { return b.size }

func (b *partBuffer) Full() bool { return b.size == b.limit }

// Append copies as much data as fits and returns the number of bytes consumed.
func (b *partBuffer) Append(data []byte) int {
	if b.released || len(data) == 0 || b.Full() {
		return 0
	}
	written := 0
	for len(data) > 0 && !b.Full() {
		if len(b.chunks) == 0 || len(b.chunks[len(b.chunks)-1]) == cap(b.chunks[len(b.chunks)-1]) {
			remaining := min(b.limit-b.size, len(data))
			chunk := b.pool.take(remaining, b.size < bufferChunkSize)
			b.chunks = append(b.chunks, chunk[:0])
		}
		last := len(b.chunks) - 1
		count := min(cap(b.chunks[last])-len(b.chunks[last]), len(data), b.limit-b.size)
		b.chunks[last] = append(b.chunks[last], data[:count]...)
		b.size += count
		written += count
		data = data[count:]
	}
	return written
}

func (b *partBuffer) Read(dst []byte) (int, error) {
	if b.released {
		return 0, errors.New("s3 sink: read released part buffer")
	}
	if b.position >= int64(b.size) {
		return 0, io.EOF
	}
	written := 0
	for written < len(dst) && b.readChunk < len(b.chunks) {
		chunk := b.chunks[b.readChunk]
		if b.readOffset == len(chunk) {
			b.readChunk++
			b.readOffset = 0
			continue
		}
		count := copy(dst[written:], chunk[b.readOffset:])
		written += count
		b.readOffset += count
	}
	b.position += int64(written)
	return written, nil
}

func (b *partBuffer) Seek(offset int64, whence int) (int64, error) {
	if b.released {
		return 0, errors.New("s3 sink: seek released part buffer")
	}
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = b.position + offset
	case io.SeekEnd:
		next = int64(b.size) + offset
	default:
		return 0, errors.New("s3 sink: invalid seek origin")
	}
	if next < 0 {
		return 0, errors.New("s3 sink: negative seek position")
	}
	b.position = next
	b.readChunk = 0
	b.readOffset = 0
	remaining := next
	for b.readChunk < len(b.chunks) && remaining >= int64(len(b.chunks[b.readChunk])) {
		remaining -= int64(len(b.chunks[b.readChunk]))
		b.readChunk++
	}
	if b.readChunk < len(b.chunks) {
		b.readOffset = int(remaining)
	}
	return next, nil
}

func (b *partBuffer) Release() {
	if b == nil || b.released {
		return
	}
	b.released = true
	for _, chunk := range b.chunks {
		b.pool.put(chunk)
	}
	b.chunks = nil
	b.size = 0
}
