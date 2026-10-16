package embedding

import (
	"strconv"

	"github.com/google/uuid"
)

// Namespace is the fixed UUIDv5 namespace for Filament document IDs. It must
// never change: every ID already written to a store is derived from it.
var Namespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://filament.getgalaxy.io/embedding"))

// DeterministicUUID maps any source key to a stable UUIDv5. The same key
// always yields the same UUID, so upserts and deletes for a row address the
// same point, and distinct keys such as "001" and "1" never collide.
func DeterministicUUID(key string) uuid.UUID {
	return uuid.NewSHA1(Namespace, []byte(key))
}

// CanonicalUint64 reports whether key is the canonical base-10 form of an
// unsigned 64-bit integer. "42" is canonical; "042", "+42" and " 42" are not,
// so passing them through would merge keys the source treats as distinct.
func CanonicalUint64(key string) (uint64, bool) {
	v, err := strconv.ParseUint(key, 10, 64)
	if err != nil || strconv.FormatUint(v, 10) != key {
		return 0, false
	}
	return v, true
}

// CanonicalUUID reports whether key is a UUID in canonical form: lowercase,
// hyphenated, 36 characters. Other spellings of the same UUID are rejected
// for the same reason as CanonicalUint64.
func CanonicalUUID(key string) (uuid.UUID, bool) {
	v, err := uuid.Parse(key)
	if err != nil || v.String() != key {
		return uuid.UUID{}, false
	}
	return v, true
}
