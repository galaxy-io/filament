package mysql

import "testing"

// keyOf compares an update's before and after key images. Two distinct
// composite keys whose values contain the old 0x1f join separator rendered
// as the same string, so a key-changing update looked like an in-place
// update and the old key's row was never deleted.
func TestKeyOfSeparatesCollidingCompositeKeys(t *testing.T) {
	dec := newRowDecoder("t", []column{
		{name: "a", dataType: "varchar", fullType: "varchar(64)"},
		{name: "b", dataType: "varchar", fullType: "varchar(64)"},
	}, []pkColumn{{name: "a"}, {name: "b"}})

	before := []any{"a\u001f", "b"}
	after := []any{"a", "\u001fb"}
	if dec.keyOf(before) == dec.keyOf(after) {
		t.Fatalf("distinct keys %q and %q render as the same key %q", before, after, dec.keyOf(before))
	}
	if dec.keyOf(before) != dec.keyOf([]any{"a\u001f", "b"}) {
		t.Fatal("identical key images render differently")
	}
	if dec.keyOf([]any{"ab", "c"}) == dec.keyOf([]any{"a", "bc"}) {
		t.Fatal("control keys (\"ab\",\"c\") and (\"a\",\"bc\") render as the same key")
	}
}
