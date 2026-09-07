//go:build integration || e2e

package testutil

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// MustStruct converts a test configuration map into its API representation.
func MustStruct(t testing.TB, values map[string]any) *structpb.Struct {
	t.Helper()
	value, err := structpb.NewStruct(values)
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	return value
}
