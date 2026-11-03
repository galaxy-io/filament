package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestExtractRejectsIntegerOutsideDeclaredWidth(t *testing.T) {
	for _, tc := range []struct{ typ, value string }{
		{"int16", "-32769"},
		{"int16", "32768"},
		{"int32", "-2147483649"},
		{"int32", "2147483648"},
	} {
		t.Run(tc.typ+" "+tc.value, func(t *testing.T) {
			rows, err := extractValue(t, tc.typ, tc.value)
			if !errors.Is(err, strconv.ErrRange) || !strings.Contains(err.Error(), `field "n"`) {
				t.Fatalf("got rows %q, err %v, want a range error for field \"n\"", rows, err)
			}
		})
	}
}

func TestExtractKeepsIntegerWithinDeclaredWidth(t *testing.T) {
	for _, tc := range []struct{ typ, value, want string }{
		{"int16", "-32768", "-32768"},
		{"int16", "32767", "32767"},
		{"int16", "42", "42"},
		{"int16", `"42"`, "42"},
		{"int16", "1.0", "1"},
		{"int16", "1e2", "100"},
		{"int32", "-2147483648", "-2147483648"},
		{"int32", "2147483647", "2147483647"},
		{"int64", "9223372036854775807", "9223372036854775807"},
	} {
		t.Run(tc.typ+" "+tc.value, func(t *testing.T) {
			rows, err := extractValue(t, tc.typ, tc.value)
			want := []string{fmt.Sprintf(`{"id":"a","n":%s}`, tc.want)}
			if err != nil || !slices.Equal(rows, want) {
				t.Fatalf("got rows %q, err %v, want %q", rows, err, want)
			}
		})
	}
}

func TestExtractFixedDecimalRounding(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, value, want string
		wantErr                        bool
	}{
		{"strict", `{type: decimal, precision: 38, scale: 9}`, "33.3333333333", "", true},
		{"extra digits", `{type: decimal, precision: 38, scale: 9, rounding: half_even}`, "33.3333333333", "33.333333333", false},
		{"half to even", `{type: decimal, precision: 3, scale: 2, rounding: half_even}`, "1.245", "1.24", false},
		{"half away from odd", `{type: decimal, precision: 3, scale: 2, rounding: half_even}`, "1.255", "1.26", false},
		{"negative half", `{type: decimal, precision: 3, scale: 2, rounding: half_even}`, "-1.255", "-1.26", false},
		{"rounded overflow", `{type: decimal, precision: 3, scale: 2, rounding: half_even}`, "9.995", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := extractValue(t, tc.declaration, tc.value)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), `field "n"`) {
					t.Fatalf("rows=%q, err=%v, want decimal field error", rows, err)
				}
				return
			}
			want := []string{fmt.Sprintf(`{"id":"a","n":%s}`, tc.want)}
			if err != nil || !slices.Equal(rows, want) {
				t.Fatalf("rows=%q, err=%v, want %q", rows, err, want)
			}
		})
	}
}

// extractValue extracts one record whose field n is declared as typ and holds
// the JSON value, and returns the rows read back from the Arrow batch.
func extractValue(t *testing.T, typ, value string) ([]string, error) {
	t.Helper()
	ctx := context.Background()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"id":"a","n":%s}]`, value)
	}))
	defer api.Close()

	path := filepath.Join(t.TempDir(), "manifest.yaml")
	data := fmt.Sprintf(`version: 1
name: widths
display_name: Widths
description: Test integer widths.
dark_logo_url: https://cdn.example.com/widths-dark.svg
light_logo_url: https://cdn.example.com/widths-light.svg
connection:
  base_url: %s
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields:
      id: string
      n: %s
    response:
      records: $
      pagination: none
`, api.URL, typ)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"manifest_path": path})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	rows := make([]string, len(sink.records))
	for i, rec := range sink.records {
		rows[i] = string(rec.Data)
	}
	return rows, err
}
