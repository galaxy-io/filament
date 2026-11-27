package mysql

import (
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/arrowtext"
	"github.com/galaxy-io/filament/rowmodel"
)

// stubRowWriter implements arrowbatch.RowWriter for unit testing parseDate and parseDatetime.
type stubRowWriter struct {
	nullCalled bool
	dateVal    int32
	timeVal    int64
	tsVal      int64
}

func (s *stubRowWriter) Null()                      { s.nullCalled = true }
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
func (s *stubRowWriter) Date(v int32)               { s.dateVal = v }
func (s *stubRowWriter) Time(v int64)               { s.timeVal = v }
func (s *stubRowWriter) Timestamp(v int64)          { s.tsVal = v }
func (s *stubRowWriter) EndRow(rowmodel.Meta) error { return nil }
func (s *stubRowWriter) Flush() error               { return nil }
func (s *stubRowWriter) Drain(rowmodel.Meta) error  { return nil }
func (s *stubRowWriter) Close() error               { return nil }

var _ arrowbatch.RowWriter = (*stubRowWriter)(nil)

// TestParseDate verifies text and binary protocol parsing for valid dates, zero dates, and invalid formats.
func TestParseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNull bool
		wantDate int32
		wantErr  bool
	}{
		{
			name:     "text zero date",
			input:    "0000-00-00",
			wantNull: true,
		},
		{
			name:     "binary-protocol rfc3339 zero date",
			input:    "0000-00-00T00:00:00Z",
			wantNull: true,
		},
		{
			name:     "text partial zero date with zero day",
			input:    "2024-05-00",
			wantNull: true,
		},
		{
			name:     "text partial zero date with zero month",
			input:    "2024-00-15",
			wantNull: true,
		},
		{
			name:     "binary-protocol rfc3339 partial zero date",
			input:    "2024-05-00T00:00:00Z",
			wantNull: true,
		},
		{
			name:     "valid text date",
			input:    "2024-05-01",
			wantDate: int32(arrowtext.DateToDays(2024, 5, 1)),
		},
		{
			name:     "valid binary-protocol date",
			input:    "2024-05-01T00:00:00Z",
			wantDate: int32(arrowtext.DateToDays(2024, 5, 1)),
		},
		{
			name:    "invalid format",
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "malformed prefix zero date",
			input:   "0000-00-00garbage",
			wantErr: true,
		},
		{
			name:    "day past the end of the month",
			input:   "2024-02-31",
			wantErr: true,
		},
		{
			name:    "month out of range",
			input:   "2024-13-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &stubRowWriter{}
			err := parseDate(w, []byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNull && !w.nullCalled {
				t.Fatalf("expected Null() to be called")
			}
			if !tt.wantNull {
				if w.nullCalled {
					t.Fatalf("expected Null() not to be called")
				}
				if w.dateVal != tt.wantDate {
					t.Fatalf("got date %d, want %d", w.dateVal, tt.wantDate)
				}
			}
		})
	}
}

// TestParseDatetime verifies text and binary protocol parsing for valid datetimes, zero datetimes, and invalid formats.
func TestParseDatetime(t *testing.T) {
	wantTs := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC).UnixMicro()

	tests := []struct {
		name     string
		input    string
		wantNull bool
		wantTs   int64
		wantErr  bool
	}{
		{
			name:     "text zero datetime",
			input:    "0000-00-00 00:00:00",
			wantNull: true,
		},
		{
			name:     "binary-protocol rfc3339 zero datetime",
			input:    "0000-00-00T00:00:00Z",
			wantNull: true,
		},
		{
			name:     "text partial zero datetime with zero day",
			input:    "2024-05-00 12:00:00",
			wantNull: true,
		},
		{
			name:     "text partial zero datetime with zero month",
			input:    "2024-00-15 12:00:00",
			wantNull: true,
		},
		{
			name:     "valid text datetime",
			input:    "2024-05-01 12:00:00",
			wantNull: false,
			wantTs:   wantTs,
		},
		{
			name:     "valid binary-protocol datetime",
			input:    "2024-05-01T12:00:00Z",
			wantNull: false,
			wantTs:   wantTs,
		},
		{
			name:    "invalid datetime",
			input:   "invalid-datetime",
			wantErr: true,
		},
		{
			name:    "minute out of range",
			input:   "2024-05-01 12:60:00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &stubRowWriter{}
			err := parseDatetime(w, []byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNull && !w.nullCalled {
				t.Fatalf("expected Null() to be called")
			}
			if !tt.wantNull {
				if w.nullCalled {
					t.Fatalf("expected Null() not to be called")
				}
				if w.tsVal != tt.wantTs {
					t.Fatalf("got ts %d, want %d", w.tsVal, tt.wantTs)
				}
			}
		})
	}
}

// TestParseTime verifies the full MySQL TIME range: an ordinary time of
// day, values past a day up to the 838:59:59 limit, and negative values
// down to -838:59:59, all of which MySQL stores and the read must not
// fail on. Values outside the documented range still error.
func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64 // signed microseconds
		wantErr bool
	}{
		// Ordinary times of day, unchanged.
		{name: "midday", input: "13:00:00", want: 13 * arrowtext.MicrosPerHour},
		{name: "midnight", input: "00:00:00", want: 0},
		{name: "last in-day second", input: "23:59:59", want: 86_399 * arrowtext.MicrosPerSecond},
		{name: "single-digit hour", input: "1:00:00", want: arrowtext.MicrosPerHour},
		{name: "fraction", input: "01:00:00.5", want: arrowtext.MicrosPerHour + 500_000},
		// The issue's cases: past a day, up to the MySQL limit.
		{name: "a full day", input: "24:00:00", want: arrowtext.MicrosPerDay},
		{name: "just past a day", input: "24:00:01", want: arrowtext.MicrosPerDay + arrowtext.MicrosPerSecond},
		{name: "a hundred hours", input: "100:00:00", want: 100 * arrowtext.MicrosPerHour},
		{name: "the maximum", input: "838:59:59", want: 3_020_399 * arrowtext.MicrosPerSecond},
		{name: "the maximum with fraction", input: "838:59:59.999999", want: 3_020_399_999_999},
		// Negative values, down to the MySQL limit.
		{name: "negative hour", input: "-01:00:00", want: -arrowtext.MicrosPerHour},
		{name: "negative sub-hour", input: "-00:30:00", want: -30 * arrowtext.MicrosPerMinute},
		{name: "negative fraction", input: "-00:00:00.5", want: -500_000},
		{name: "negative zero", input: "-00:00:00", want: 0},
		{name: "the negative maximum", input: "-838:59:59", want: -3_020_399 * arrowtext.MicrosPerSecond},
		{name: "the negative maximum with fraction", input: "-838:59:59.999999", want: -3_020_399_999_999},
		// Outside the documented range, or not a time at all.
		{name: "past the maximum", input: "839:00:00", wantErr: true},
		{name: "past the negative maximum", input: "-839:00:00", wantErr: true},
		{name: "four hour digits", input: "1000:00:00", wantErr: true},
		{name: "minute of sixty", input: "12:60:00", wantErr: true},
		{name: "second of sixty", input: "12:00:60", wantErr: true},
		{name: "plus sign", input: "+01:00:00", wantErr: true},
		{name: "trailing text", input: "01:00:00garbage", wantErr: true},
		{name: "fraction dot without digits", input: "01:00:00.", wantErr: true},
		{name: "missing seconds", input: "12:00", wantErr: true},
		{name: "sign only", input: "-", wantErr: true},
		{name: "not a time", input: "not-a-time", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &stubRowWriter{}
			err := parseTime(w, []byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w.timeVal != tt.want {
				t.Fatalf("got time %d, want %d", w.timeVal, tt.want)
			}
		})
	}
}
