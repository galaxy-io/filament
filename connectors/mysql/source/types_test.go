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
func (s *stubRowWriter) Time(int64)                 {}
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
