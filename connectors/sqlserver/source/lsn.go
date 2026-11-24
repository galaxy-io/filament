package sqlserver

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

// lsn is a log sequence number: the binary(10) form CDC stores, a virtual log
// file sequence (4 bytes), a block offset (4) and a slot (2), all big-endian.
// Byte order is log order, so LSNs compare bytewise.
type lsn [10]byte

// String renders the LSN as 20 lowercase hex digits. The fixed width keeps
// the text form in log order too.
func (l lsn) String() string { return hex.EncodeToString(l[:]) }

func (l lsn) compare(o lsn) int { return bytes.Compare(l[:], o[:]) }

func (l lsn) isZero() bool { return l == lsn{} }

// prev is the LSN just before l. A change row is stamped with the LSN before
// its transaction's commit, so a checkpoint taken mid-transaction resumes by
// re-reading the whole transaction rather than skipping its remaining rows.
func (l lsn) prev() lsn {
	for i := len(l) - 1; i >= 0; i-- {
		if l[i] > 0 {
			l[i]--
			return l
		}
		l[i] = 0xff
	}
	return lsn{}
}

// next is the LSN just after l: the first a read after l can return.
func (l lsn) next() lsn {
	for i := len(l) - 1; i >= 0; i-- {
		if l[i] < 0xff {
			l[i]++
			return l
		}
		l[i] = 0
	}
	return lsn{}
}

func maxLSN(a, b lsn) lsn {
	if a.compare(b) >= 0 {
		return a
	}
	return b
}

func minLSN(a, b lsn) lsn {
	if a.compare(b) <= 0 {
		return a
	}
	return b
}

// lsnFromBytes reads a binary(10) value; NULL (nil) is the zero LSN.
func lsnFromBytes(b []byte) (lsn, error) {
	var l lsn
	switch len(b) {
	case 0:
		return l, nil
	case len(l):
		copy(l[:], b)
		return l, nil
	}
	return l, fmt.Errorf("lsn of %d bytes, want %d", len(b), len(l))
}

// parseLSN reads the checkpoint form (20 hex digits, optionally 0x-prefixed)
// and the form DMVs such as sys.dm_db_log_stats report, three colon-separated
// hex fields "0000002B:00000F38:0001".
func parseLSN(s string) (lsn, error) {
	var l lsn
	text := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if parts := strings.Split(text, ":"); len(parts) == 3 {
		if len(parts[0]) > 8 || len(parts[1]) > 8 || len(parts[2]) > 4 {
			return l, fmt.Errorf("invalid lsn %q", s)
		}
		text = padHex(parts[0], 8) + padHex(parts[1], 8) + padHex(parts[2], 4)
	}
	if len(text) != 2*len(l) {
		return l, fmt.Errorf("invalid lsn %q", s)
	}
	if _, err := hex.Decode(l[:], []byte(text)); err != nil {
		return l, fmt.Errorf("invalid lsn %q: %w", s, err)
	}
	return l, nil
}

func padHex(s string, width int) string { return strings.Repeat("0", width-len(s)) + s }
