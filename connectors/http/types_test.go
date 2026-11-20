package httpapi

import (
	"context"
	"encoding/json"
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

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
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

// stubRowWriter implements arrowbatch.RowWriter for unit testing the value
// parsers; it records only the timestamp value.
type stubRowWriter struct {
	tsVal int64
}

func (s *stubRowWriter) Null()                      {}
func (s *stubRowWriter) Bool(bool)                  {}
func (s *stubRowWriter) Int16(int16)                {}
func (s *stubRowWriter) Int32(int32)                {}
func (s *stubRowWriter) Int64(int64)                {}
func (s *stubRowWriter) Float32(float32)            {}
func (s *stubRowWriter) Float64(float64)            {}
func (s *stubRowWriter) Decimal(decimal128.Num)     {}
func (s *stubRowWriter) String(string)              {}
func (s *stubRowWriter) StringBytes([]byte)         {}
func (s *stubRowWriter) Bytes([]byte)               {}
func (s *stubRowWriter) Date(int32)                 {}
func (s *stubRowWriter) Time(int64)                 {}
func (s *stubRowWriter) Timestamp(v int64)          { s.tsVal = v }
func (s *stubRowWriter) EndRow(rowmodel.Meta) error { return nil }
func (s *stubRowWriter) Flush() error               { return nil }
func (s *stubRowWriter) Drain(rowmodel.Meta) error  { return nil }
func (s *stubRowWriter) Close() error               { return nil }

var _ arrowbatch.RowWriter = (*stubRowWriter)(nil)

// TestParseTimestampEpochUnits verifies that a numeric epoch value is read
// in the unit its magnitude implies: seconds, milliseconds or microseconds.
// Epoch milliseconds for any date before September 2001 are below 1e12, so
// a single threshold at 1e12 reads them as seconds and lands the instant
// roughly a thousand times too far in the future.
func TestParseTimestampEpochUnits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64 // microseconds since the epoch
	}{
		// The issue's cases: epoch milliseconds before September 2001.
		{"millis for 2000-01-01", "946684800000", 946_684_800_000_000},
		{"millis for 2001-09-09 boundary", "1000000000000", 1_000_000_000_000_000},
		{"millis just below the boundary", "999999999999", 999_999_999_999_000},
		{"millis just above the boundary", "1000000000001", 1_000_000_000_001_000},
		// Controls from the issue.
		{"seconds for 2025-01-01", "1735689600", 1_735_689_600_000_000},
		{"millis for 2025-01-01", "1735689600000", 1_735_689_600_000_000},
		// Band edges: adjacent inputs either side of a boundary take
		// different units.
		{"largest seconds value", "9999999999", 9_999_999_999_000_000},
		{"smallest millis value", "10000000000", 10_000_000_000_000},
		{"largest millis value", "9999999999999", 9_999_999_999_999_000},
		{"smallest micros value", "10000000000000", 10_000_000_000_000},
		// Microseconds.
		{"micros for 2025-01-01", "1735689600000000", 1_735_689_600_000_000},
		// Negative values band on magnitude: pre-epoch dates arrive in
		// the same units as post-epoch ones.
		{"negative seconds", "-1", -1_000_000},
		{"negative millis", "-946684800000", -946_684_800_000_000},
		// Fractional values keep their fraction in every unit.
		{"fractional seconds", "1735689600.5", 1_735_689_600_500_000},
		{"fractional millis", "946684800000.25", 946_684_800_000_250},
		// Zero and exponent spellings.
		{"zero", "0", 0},
		{"millis in exponent form", "9.466848e11", 946_684_800_000_000},
		// A quoted ISO string still takes the string path.
		{"iso string", `"2025-01-01T00:00:00Z"`, 1_735_689_600_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &stubRowWriter{}
			if err := parseTimestamp(w, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("parseTimestamp(%s): unexpected error: %v", tt.input, err)
			}
			if w.tsVal != tt.want {
				t.Fatalf("parseTimestamp(%s) = %d, want %d", tt.input, w.tsVal, tt.want)
			}
		})
	}
	if err := parseTimestamp(&stubRowWriter{}, json.RawMessage(`not-a-number`)); err == nil {
		t.Fatalf("parseTimestamp(not-a-number): expected an error, got nil")
	}
}
