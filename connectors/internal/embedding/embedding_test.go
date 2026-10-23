package embedding

import (
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

// TestParseFloats verifies valid float list parsing and error handling for malformed input.
func TestParseFloats(t *testing.T) {
	floats, err := ParseFloats("[0.1, 0.25, 0.5]")
	if err != nil {
		t.Fatalf("unexpected error parsing valid floats: %v", err)
	}
	if len(floats) != 3 {
		t.Fatalf("expected 3 floats, got %d", len(floats))
	}
	if floats[0] != 0.1 || floats[1] != 0.25 || floats[2] != 0.5 {
		t.Errorf("unexpected float values: %v", floats)
	}

	_, err = ParseFloats("[0.1, invalid_token, 0.5]")
	if err == nil {
		t.Errorf("expected ParseFloats to return an error on invalid token, got nil")
	}
}

// TestOpString verifies Operation enum string conversion.
func TestOpString(t *testing.T) {
	str, err := OpString(rowmodel.OpInsert)
	if err != nil || str != "insert" {
		t.Errorf("expected 'insert', got %q (err: %v)", str, err)
	}

	str, err = OpString(rowmodel.OpUpdate)
	if err != nil || str != "update" {
		t.Errorf("expected 'update', got %q (err: %v)", str, err)
	}

	str, err = OpString(rowmodel.OpDelete)
	if err != nil || str != "delete" {
		t.Errorf("expected 'delete', got %q (err: %v)", str, err)
	}

	_, err = OpString(rowmodel.Operation(99))
	if err == nil {
		t.Errorf("expected error for invalid operation enum, got nil")
	}
}
