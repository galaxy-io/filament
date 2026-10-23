package schema

import (
	"fmt"
	"unicode/utf8"

	"github.com/galaxy-io/filament/rowmodel"
)

// Validate checks structural identity, not engine-specific type validity or
// reserved-field permission. Empty/unknown logical types remain representable.
func Validate(s rowmodel.Schema) error {
	for _, value := range []string{s.Resource, s.DestinationResource, s.Engine} {
		if !utf8.ValidString(value) {
			return fmt.Errorf("schema contains invalid UTF-8")
		}
	}
	names := make(map[string]bool, len(s.Fields))
	for _, f := range s.Fields {
		if f.Name == "" || names[f.Name] {
			return fmt.Errorf("schema has empty or duplicate field %q", f.Name)
		}
		if !utf8.ValidString(f.Name) || !utf8.ValidString(string(f.Logical)) || !utf8.ValidString(f.Native) {
			return fmt.Errorf("field %q contains invalid UTF-8", f.Name)
		}
		names[f.Name] = true
	}
	keys := make(map[string]bool, len(s.PrimaryKey))
	for _, key := range s.PrimaryKey {
		if !names[key] || keys[key] {
			return fmt.Errorf("schema has missing or duplicate key field %q", key)
		}
		keys[key] = true
	}
	return nil
}
