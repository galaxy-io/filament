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

func TestExtractUnboundedDecimalExponent(t *testing.T) {
	// An unbounded decimal in exponent form must keep its exact digits.
	// A float64 round-trip only holds about 15 to 17 significant digits,
	// so longer values lost digits silently. See issue #740.
	for _, tc := range []struct{ value, want string }{
		{"1.234567890123456789e10", "12345678901.23456789"},
		{"1.234567890123456789e20", "123456789012345678900"},
		{"-1.234567890123456789e10", "-12345678901.23456789"},
		{"1.5e-3", "0.0015"},
		{"1e5", "100000"},
		{"1.234567890123456789", "1.234567890123456789"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			rows, err := extractValue(t, "decimal", tc.value)
			want := []string{fmt.Sprintf(`{"id":"a","n":%s}`, tc.want)}
			if err != nil || !slices.Equal(rows, want) {
				t.Fatalf("got rows %q, err %v, want %q", rows, err, want)
			}
		})
	}
}

func TestExtractUnboundedDecimalExponentExact(t *testing.T) {
	// The expansion works on the digits directly, never through a binary
	// float, so even a value with more significant digits than any float
	// mantissa holds keeps them all. See issue #740 and the CodeRabbit
	// review on PR #744.
	longDigits := "1." + strings.Repeat("0", 80) + "1e0"
	for _, tc := range []struct{ value, want string }{
		{longDigits, "1." + strings.Repeat("0", 80) + "1"},
		{"1.5e3", "1500"},
		{"1.50e0", "1.5"},
		{"-0.25e-2", "-0.0025"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			rows, err := extractValue(t, "decimal", tc.value)
			want := []string{fmt.Sprintf(`{"id":"a","n":%s}`, tc.want)}
			if err != nil || !slices.Equal(rows, want) {
				t.Fatalf("got rows %q, err %v, want %q", rows, err, want)
			}
		})
	}
}

func TestExtractUnboundedDecimalExponentBound(t *testing.T) {
	// A value whose plain digit expansion would exceed the bound is an
	// error, not a huge allocation. See the CodeRabbit review on PR #744.
	for _, value := range []string{"1e100000000", "1e10001", "1e-100000"} {
		t.Run(value, func(t *testing.T) {
			rows, err := extractValue(t, "decimal", value)
			if err == nil || !strings.Contains(err.Error(), `field "n"`) {
				t.Fatalf("rows=%q, err=%v, want decimal field error", rows, err)
			}
		})
	}
}

func TestExtractTimestampAcceptsNumericOffset(t *testing.T) {
	// Jira Cloud emits created and updated with a numeric zone offset and no
	// colon, for example 2026-11-13T01:09:44.630+0500. RFC 3339 parsing
	// rejects that spelling, so the timestamp layouts carry it explicitly.
	for _, tc := range []struct{ value, wantPrefix string }{
		{`"2026-11-13T01:09:44.630+0500"`, "2026-11-12T20:09:44"},
		{`"2026-11-13T01:09:44.630-0500"`, "2026-11-13T06:09:44"},
		{`"2026-11-13T01:09:44.630+05:00"`, "2026-11-12T20:09:44"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			rows, err := extractValue(t, "timestamptz", tc.value)
			if err != nil {
				t.Fatalf("rows=%q, err=%v, want a parsed timestamp", rows, err)
			}
			if len(rows) != 1 || !strings.Contains(rows[0], tc.wantPrefix) {
				t.Fatalf("rows=%q, want the UTC instant starting %q", rows, tc.wantPrefix)
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
