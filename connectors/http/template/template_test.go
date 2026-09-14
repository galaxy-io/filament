package template

import (
	"reflect"
	"testing"
)

func TestSplitBodyValue(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  []any
	}{
		{" name, custom_field ,", []any{"name", "custom_field"}},
		{"", []any{}},
		{"a\"b,東京", []any{"a\"b", "東京"}},
	} {
		got, err := RenderAny(`{{ config.properties | split "," }}`, Scope{Config: map[string]string{"properties": tc.value}})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%q: %v, want %v", tc.value, got, tc.want)
		}
	}
	if _, err := RenderAny(`prefix {{ config.properties | split "," }}`, Scope{Config: map[string]string{"properties": "name"}}); err == nil {
		t.Fatal("split embedded in string must fail")
	}
	got, err := RenderAny(`{{ config.missing | default "fallback" }}`, Scope{})
	if err != nil || got != "fallback" {
		t.Fatalf("default behavior changed: %v %v", got, err)
	}
}
