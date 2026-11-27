package s3

import (
	"context"
	"hash/crc32"
	"slices"
	"testing"
)

// TestPartLimitReachesObjectLimit proves every accepted part_size_mib can carry
// an object close to the S3 5 TiB limit inside 10,000 parts, with every part
// inside the S3 per-part bounds.
func TestPartLimitReachesObjectLimit(t *testing.T) {
	const floor = int64(4_800) << 30 // ten 5 MiB-based tiers reach ~4.88 TiB
	for _, mib := range []int{minPartSizeMiB, defaultPartSizeMiB, 64, 1024, maxPartSizeMiB} {
		base := mib << 20
		var total int64
		previous := 0
		for n := int32(1); n <= maxUploadParts; n++ {
			size := partLimit(base, n, partsPerTier)
			if size < minPartSize || size > maxPartSize {
				t.Fatalf("base %d MiB part %d is %d bytes, outside S3 bounds", mib, n, size)
			}
			if size < previous {
				t.Fatalf("base %d MiB part %d shrank from %d to %d", mib, n, previous, size)
			}
			previous = size
			total += int64(size)
		}
		if total < floor {
			t.Fatalf("base %d MiB reaches %d bytes in %d parts, want at least %d", mib, total, maxUploadParts, floor)
		}
		if got := partLimit(base, 1, partsPerTier); got != base {
			t.Fatalf("base %d MiB first part is %d bytes", mib, got)
		}
	}
}

// TestMultipartSessionGrowsParts streams through a session with a short tier so
// the growth is visible, and checks the store saw exactly the scheduled part
// sizes and the bytes the session reported.
func TestMultipartSessionGrowsParts(t *testing.T) {
	ctx := context.Background()
	store := newLimitStore()
	session := newMultipartSession(ctx, store, "bucket", minPartSize, 2, objectMetadata{})
	session.perTier = 2

	const mib = 1 << 20
	want := []int64{5 * mib, 5 * mib, 10 * mib, 10 * mib, 20 * mib, 3 * mib}
	var total int64
	for _, size := range want {
		total += size
	}
	chunk := make([]byte, 768<<10) // not a divisor of any part size
	for i := range chunk {
		chunk[i] = byte(i * 31)
	}
	var crc uint32
	for written := int64(0); written < total; {
		data := chunk[:min(int64(len(chunk)), total-written)]
		piece := crc32.Checksum(data, castagnoli)
		if err := session.Append(ctx, "r", "key", data, 1, piece); err != nil {
			t.Fatal(err)
		}
		crc = crc32.Update(crc, castagnoli, data)
		written += int64(len(data))
	}
	results, err := session.Complete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := store.object("key")
	if !ok {
		t.Fatal("object was not completed")
	}
	if !slices.Equal(got.partSizes, want) {
		t.Fatalf("part sizes = %v, want %v", got.partSizes, want)
	}
	if got.size != total || got.crc != crc {
		t.Fatalf("stored %d bytes crc %08x, want %d crc %08x", got.size, got.crc, total, crc)
	}
	if len(results) != 1 || results[0].Bytes != total || results[0].CRC32C != crc {
		t.Fatalf("result = %#v", results)
	}
	if store.pending() != 0 {
		t.Fatalf("%d multipart uploads left open", store.pending())
	}
}
