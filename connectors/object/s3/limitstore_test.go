package s3

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	object "github.com/galaxy-io/filament/connectors/object/internal"
)

const minPartSize = minPartSizeMiB << 20

var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// limitStore is an in-memory multipartStore that enforces the S3 multipart
// rules a real bucket enforces, including rejecting a part, multipart object,
// or single object whose CRC32C does not match the bytes received. It keeps
// sizes and checksums, and keeps the bytes only when keep is set.
type limitStore struct {
	keep bool // retain object bodies for tests that decode them

	nextID atomic.Int64

	mu         sync.Mutex
	uploads    map[string]*fakeUpload
	objects    map[string]fakeObject
	overwrites []string // keys written more than once; a bucket replaces them silently
}

type fakeUpload struct {
	key   string
	parts map[int32]fakePart
}

type fakePart struct {
	size int64
	crc  uint32
	body []byte
}

type fakeObject struct {
	size      int64
	crc       uint32
	partSizes []int64 // nil for a single PutObject
	body      []byte  // set only when the store keeps bodies
}

var _ multipartStore = (*limitStore)(nil)

func newLimitStore() *limitStore {
	return &limitStore{uploads: make(map[string]*fakeUpload), objects: make(map[string]fakeObject)}
}

func (s *limitStore) readBody(body io.Reader, size int64) (uint32, []byte, error) {
	hash := crc32.New(castagnoli)
	var kept bytes.Buffer
	sink := io.Writer(hash)
	if s.keep {
		sink = io.MultiWriter(hash, &kept)
	}
	read, err := io.Copy(sink, body)
	if err != nil {
		return 0, nil, err
	}
	if read != size {
		return 0, nil, fmt.Errorf("body has %d bytes, content length %d", read, size)
	}
	if !s.keep {
		return hash.Sum32(), nil, nil
	}
	return hash.Sum32(), kept.Bytes(), nil
}

func (s *limitStore) CreateMultipart(ctx context.Context, _, key string, _ objectMetadata) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	id := strconv.FormatInt(s.nextID.Add(1), 10)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[id] = &fakeUpload{key: key, parts: make(map[int32]fakePart)}
	return id, nil
}

func (s *limitStore) UploadPart(ctx context.Context, _, key, uploadID string, number int32, body io.ReadSeeker, size int64, sent uint32) (string, error) {
	if number < 1 || number > maxUploadParts {
		return "", fmt.Errorf("part number %d outside 1..%d", number, maxUploadParts)
	}
	if size > maxPartSize {
		return "", fmt.Errorf("part %d is %d bytes, over the %d byte limit", number, size, maxPartSize)
	}
	crc, kept, err := s.readBody(body, size)
	if err != nil {
		return "", err
	}
	if crc != sent {
		return "", fmt.Errorf("part %d CRC32C mismatch: sent %08x, received %08x", number, sent, crc)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	upload := s.uploads[uploadID]
	if upload == nil || upload.key != key {
		return "", fmt.Errorf("no such upload %q for %q", uploadID, key)
	}
	upload.parts[number] = fakePart{size: size, crc: crc, body: kept}
	return strconv.Itoa(int(number)), nil
}

func (s *limitStore) CompleteMultipart(ctx context.Context, _, key, uploadID string, parts []completedPart, sent uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	upload := s.uploads[uploadID]
	if upload == nil || upload.key != key {
		return fmt.Errorf("no such upload %q for %q", uploadID, key)
	}
	if len(parts) == 0 {
		return fmt.Errorf("complete %q with no parts", key)
	}
	out := fakeObject{partSizes: make([]int64, 0, len(parts))}
	for i, part := range parts {
		if i > 0 && part.number <= parts[i-1].number {
			return fmt.Errorf("parts out of order at %d", part.number)
		}
		stored, ok := upload.parts[part.number]
		if !ok || part.token != strconv.Itoa(int(part.number)) {
			return fmt.Errorf("part %d was not uploaded", part.number)
		}
		if part.crc32c != stored.crc {
			return fmt.Errorf("part %d CRC32C mismatch at completion", part.number)
		}
		if i < len(parts)-1 && stored.size < minPartSize {
			return fmt.Errorf("part %d is %d bytes; only the last part may be under %d", part.number, stored.size, minPartSize)
		}
		out.crc = object.CombineCRC32C(out.crc, stored.crc, stored.size)
		out.size += stored.size
		out.partSizes = append(out.partSizes, stored.size)
		if s.keep {
			out.body = append(out.body, stored.body...)
		}
	}
	if out.crc != sent {
		return fmt.Errorf("object CRC32C mismatch: sent %08x, stored %08x", sent, out.crc)
	}
	delete(s.uploads, uploadID)
	s.store(key, out)
	return nil
}

func (s *limitStore) AbortMultipart(_ context.Context, _, _, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.uploads, uploadID)
	return nil
}

func (s *limitStore) PutObject(ctx context.Context, _, key string, _ objectMetadata, body io.ReadSeeker, size int64, sent uint32) error {
	if size > maxPartSize {
		return fmt.Errorf("single PutObject of %d bytes", size)
	}
	crc, kept, err := s.readBody(body, size)
	if err != nil {
		return err
	}
	if crc != sent {
		return fmt.Errorf("object CRC32C mismatch: sent %08x, received %08x", sent, crc)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store(key, fakeObject{size: size, crc: crc, body: kept})
	return nil
}

// store records a completed object. The caller holds s.mu.
func (s *limitStore) store(key string, out fakeObject) {
	if _, exists := s.objects[key]; exists {
		s.overwrites = append(s.overwrites, key)
	}
	s.objects[key] = out
}

func (s *limitStore) overwritten() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.overwrites)
}

func (s *limitStore) object(key string) (fakeObject, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, ok := s.objects[key]
	if ok {
		out.partSizes = slices.Clone(out.partSizes)
	}
	return out, ok
}

// keys lists completed object keys in order.
func (s *limitStore) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func (s *limitStore) pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.uploads)
}
