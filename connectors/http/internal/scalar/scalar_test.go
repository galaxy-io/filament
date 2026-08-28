package scalar

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
		ok    bool
	}{
		{name: "string", value: "value", want: "value", ok: true},
		{name: "bool", value: true, want: "true", ok: true},
		{name: "int", value: int(12), want: "12", ok: true},
		{name: "int32", value: int32(12), want: "12", ok: true},
		{name: "int64", value: int64(12), want: "12", ok: true},
		{name: "float64", value: 12.5, want: "12.5", ok: true},
		{name: "nil", value: nil},
		{name: "object", value: map[string]any{"key": "value"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := String(tt.value)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("String(%#v) = %q, %v; want %q, %v", tt.value, got, ok, tt.want, tt.ok)
			}
		})
	}
}
