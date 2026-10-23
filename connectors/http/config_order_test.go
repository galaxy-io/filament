package httpapi

import (
	"fmt"
	"slices"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestConfigFieldOrderGrammar(t *testing.T) {
	for _, order := range []string{"0", "1", "-1", "1.5", "'first'"} {
		t.Run(order, func(t *testing.T) {
			_, err := manifest.Parse([]byte(fmt.Sprintf(`version: 1
name: example
display_name: Example
description: Example source.
dark_logo_url: https://example.com/logo.svg
light_logo_url: https://example.com/logo.svg
config:
  token: {type: secret, order: %s}
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    fields: {id: string}
`, order)))
			valid := order == "0" || order == "1"
			if (err == nil) != valid {
				t.Fatalf("order %s: valid=%v, error=%v", order, valid, err)
			}
		})
	}
}

func TestConfigFieldOrder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]manifest.ConfigSpec
		want   []string
	}{
		{"default", map[string]manifest.ConfigSpec{
			"z": {Type: "string"}, "a": {Type: "string"},
		}, []string{"a", "z"}},
		{"explicit and ties", map[string]manifest.ConfigSpec{
			"a": {Type: "string"}, "b": {Type: "string", Order: 2},
			"z": {Type: "string", Order: 1}, "c": {Type: "string", Order: 2},
		}, []string{"z", "b", "c", "a"}},
		{"visibility selector", map[string]manifest.ConfigSpec{
			"mode":  {Type: "enum", Enum: []string{"custom"}},
			"token": {Type: "secret", Order: 1, VisibleWhen: &manifest.ConfigCondition{Field: "mode", Values: []string{"custom"}}},
			"other": {Type: "string", Order: 2},
		}, []string{"mode", "token", "other"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := configSchemaFromManifest(&manifest.Manifest{Config: tc.config})
			var got []string
			for _, field := range schema.Fields {
				got = append(got, field.Name)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("fields = %v, want %v", got, tc.want)
			}
		})
	}
}
