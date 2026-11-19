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
	tsVal      int64
	timeVal    int64
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


func TestParseTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTime int64
		wantText string
		wantErr  bool
	}{
		{
			name:     "standard time of day",
			input:    "13:00:00",
			wantTime: 13 * arrowtext.MicrosPerHour,
			wantText: "13:00:00",
		},
		{
			name:     "exact 24 hours",
			input:    "24:00:00",
			wantTime: 24 * arrowtext.MicrosPerHour,
			wantText: "24:00:00",
		},
		{
			name:     "three-digit hours",
			input:    "100:00:00",
			wantTime: 100 * arrowtext.MicrosPerHour,
			wantText: "100:00:00",
		},
		{
			name:     "upper bound 838:59:59",
			input:    "838:59:59",
			wantTime: 838*arrowtext.MicrosPerHour + 59*arrowtext.MicrosPerMinute + 59*arrowtext.MicrosPerSecond,
			wantText: "838:59:59",
		},
		{
			name:     "negative duration",
			input:    "-01:00:00",
			wantTime: -1 * arrowtext.MicrosPerHour,
			wantText: "-01:00:00",
		},
		{
			name:     "negative lower bound -838:59:59",
			input:    "-838:59:59",
			wantTime: -(838*arrowtext.MicrosPerHour + 59*arrowtext.MicrosPerMinute + 59*arrowtext.MicrosPerSecond),
			wantText: "-838:59:59",
		},
		{
			name:     "negative fractional duration",
			input:    "-00:00:00.5",
			wantTime: -500_000,
			wantText: "-00:00:00.5",
		},
		{
			name:    "past 838 hours",
			input:   "839:00:00",
			wantErr: true,
		},
		{
			name:    "invalid minutes",
			input:   "10:60:00",
			wantErr: true,
		},
		{
			name:    "invalid seconds",
			input:   "10:00:60",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w stubRowWriter
			err := parseTime(&w, []byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTime(%q) succeeded, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTime(%q) error: %v", tt.input, err)
			}
			if w.timeVal != tt.wantTime {
				t.Fatalf("parseTime(%q) = %d, want %d", tt.input, w.timeVal, tt.wantTime)
			}
			if gotText := string(arrowtext.AppendTimeOfDay(nil, w.timeVal)); gotText != tt.wantText {
				t.Fatalf("AppendTimeOfDay(%d) = %q, want %q", w.timeVal, gotText, tt.wantText)
			}
		})
	}
}
