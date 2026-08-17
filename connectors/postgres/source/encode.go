package postgres

// Native row encoding, the default read path. Each window selects typed
// columns over pgx's binary protocol and rows are assembled into Record.Data
// client-side, so the source database never serializes rows to JSON. The
// payload stays a JSON object semantically equivalent to to_jsonb output
// (ISO dates, numerics with exact scale, NaN/Infinity as strings), so the
// sink's typed ingestion is unchanged.
//
// A column type without a formatter here falls back to a server-side
// to_jsonb(col)::text, chosen per column at plan time, never per row, and
// embedded verbatim, keeping to_jsonb semantics for arrays, enums,
// composites, and domains.

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// The encoding config values: native column reads (default) or the legacy
// server-side to_jsonb path.
const (
	encodingNative = "native"
	encodingJSONB  = "jsonb"
)

// binaryResults asks the server for binary wire format on every result column.
// Columns the encoder embeds verbatim (text, json, the to_jsonb()::text
// fallback) are byte-identical in either format.
var binaryResults = pgx.QueryResultFormats{pgx.BinaryFormatCode}

// column is one live column of a table, from the catalog: its name and type OID.
type column struct {
	name string
	oid  uint32
}

// lookupColumns returns schema.table's live columns in attribute order — the
// same set to_jsonb(t) serializes. A zero-row probe supplies them: its
// RowDescription carries each column's name and type OID, resolving the
// relation exactly as the window reads will.
func (s *Source) lookupColumns(ctx context.Context, schema, table string) ([]column, error) {
	q := "SELECT * FROM " + pgx.Identifier{schema, table}.Sanitize() + " LIMIT 0"
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var cols []column
	for _, fd := range rows.FieldDescriptions() {
		cols = append(cols, column{name: fd.Name, oid: fd.DataTypeOID})
	}
	return cols, nil
}

// encoderFor builds table's row encoder, or nil when the jsonb fallback
// encoding is configured.
func (s *Source) encoderFor(ctx context.Context, table string, pks []string) (*rowEncoder, error) {
	if !s.nativeEncoding() {
		return nil, nil
	}
	cols, err := s.lookupColumns(ctx, s.schema, table)
	if err != nil {
		return nil, fmt.Errorf("lookup columns %q: %w", table, err)
	}
	enc, err := newRowEncoder(cols, pks)
	if err != nil {
		return nil, fmt.Errorf("encode %q: %w", table, err)
	}
	return enc, nil
}

// rowEncoder assembles one scanned row's raw binary column values into the
// record's JSON payload and id. Built once per table, shared read-only by its
// shards.
type rowEncoder struct {
	selectList string   // column projection (no ctid); fallback types wrapped in to_jsonb()::text
	keys       [][]byte // per column: its `{"name":` / `,"name":` object-key prefix
	fns        []encFns
	names      []string // column names, for error context
	pkIdx      []int    // pk column positions in key order; empty → id from a trailing ctid
}

func newRowEncoder(cols []column, pks []string) (*rowEncoder, error) {
	if len(cols) == 0 {
		return nil, fmt.Errorf("no live columns")
	}
	e := &rowEncoder{
		keys:  make([][]byte, len(cols)),
		fns:   make([]encFns, len(cols)),
		names: make([]string, len(cols)),
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		ident := "t." + pgx.Identifier{c.name}.Sanitize()
		fns, ok := encByOID[c.oid]
		if ok {
			parts[i] = ident
		} else {
			parts[i] = "to_jsonb(" + ident + ")::text"
			fns = encFns{json: encRaw, text: encJSONTextForm}
		}
		sep := byte(',')
		if i == 0 {
			sep = '{'
		}
		key, err := encJSONString([]byte{sep}, []byte(c.name))
		if err != nil {
			return nil, err
		}
		e.keys[i] = append(key, ':')
		e.fns[i] = fns
		e.names[i] = c.name
	}
	e.selectList = strings.Join(parts, ", ")
	for _, pk := range pks {
		i := slices.IndexFunc(cols, func(c column) bool { return c.name == pk })
		if i < 0 {
			return nil, fmt.Errorf("pk column %q not in catalog", pk)
		}
		e.pkIdx = append(e.pkIdx, i)
	}
	return e, nil
}

// appendRowData appends the row's payload as a JSON object in column order.
// (to_jsonb emitted the same pairs in jsonb's internal key order; JSON object
// order carries no meaning and integrity CRCs compare only within a run.)
func (e *rowEncoder) appendRowData(dst []byte, raw [][]byte) ([]byte, error) {
	if len(raw) < len(e.fns) {
		return nil, fmt.Errorf("row has %d values, want %d", len(raw), len(e.fns))
	}
	for i, fn := range e.fns {
		dst = append(dst, e.keys[i]...)
		if raw[i] == nil {
			dst = append(dst, "null"...)
			continue
		}
		var err error
		if dst, err = fn.json(dst, raw[i]); err != nil {
			return nil, fmt.Errorf("column %q: %w", e.names[i], err)
		}
	}
	return append(dst, '}'), nil
}

// pkTexts returns each pk column's text form — the keyset cursor values.
func (e *rowEncoder) pkTexts(raw [][]byte) ([]string, error) {
	out := make([]string, len(e.pkIdx))
	for n, i := range e.pkIdx {
		if raw[i] == nil {
			continue // cannot occur for a real pk; mirrors concat_ws skipping nulls
		}
		b, err := e.fns[i].text(nil, raw[i])
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", e.names[i], err)
		}
		out[n] = string(b)
	}
	return out, nil
}

// textAt renders one projected native column in the same text form PostgreSQL
// uses for keyset cursors and the jsonb compatibility projection.
func (e *rowEncoder) textAt(raw [][]byte, i int) (string, error) {
	if i < 0 || i >= len(e.fns) || i >= len(raw) {
		return "", fmt.Errorf("column index %d outside row", i)
	}
	if raw[i] == nil {
		return "", nil
	}
	b, err := e.fns[i].text(nil, raw[i])
	if err != nil {
		return "", fmt.Errorf("column %q: %w", e.names[i], err)
	}
	return string(b), nil
}

// rowID derives the record id from the pk values' text forms, or from the
// trailing ctid column for a keyless table.
func (e *rowEncoder) rowID(raw [][]byte) (string, error) {
	if len(e.pkIdx) == 0 {
		if len(raw) <= len(e.fns) {
			return "", fmt.Errorf("keyless row without ctid column")
		}
		b, err := appendTID(nil, raw[len(e.fns)])
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	texts, err := e.pkTexts(raw)
	if err != nil {
		return "", err
	}
	return joinKey(texts), nil
}

// joinKey renders pk text values as the record id: the single value, or the
// columns joined with 0x1F (the old concat_ws(chr(31), …) idExpr).
func joinKey(texts []string) string {
	if len(texts) == 1 {
		return texts[0]
	}
	return strings.Join(texts, "\x1f")
}

// encFns formats one column's binary wire value. json appends it as the JSON
// value to_jsonb would produce; text appends its unquoted text form (jsonb ->>
// semantics), used for record ids and keyset cursors.
type encFns struct {
	json func(dst, src []byte) ([]byte, error)
	text func(dst, src []byte) ([]byte, error)
}

// encByOID maps a column's type OID to its formatters. Types not listed read
// through the server-side to_jsonb fallback (see newRowEncoder).
var encByOID = map[uint32]encFns{
	pgtype.BoolOID:        {json: encBool, text: encBool},
	pgtype.Int2OID:        {json: encInt2, text: encInt2},
	pgtype.Int4OID:        {json: encInt4, text: encInt4},
	pgtype.Int8OID:        {json: encInt8, text: encInt8},
	pgtype.Float4OID:      {json: encFloat4JSON, text: encFloat4Text},
	pgtype.Float8OID:      {json: encFloat8JSON, text: encFloat8Text},
	pgtype.NumericOID:     {json: encNumericJSON, text: encNumericText},
	pgtype.TextOID:        {json: encJSONString, text: encRaw},
	pgtype.VarcharOID:     {json: encJSONString, text: encRaw},
	pgtype.BPCharOID:      {json: encJSONString, text: encRaw},
	pgtype.NameOID:        {json: encJSONString, text: encRaw},
	pgtype.ByteaOID:       {json: encByteaJSON, text: encByteaText},
	pgtype.UUIDOID:        {json: quoted(encUUIDText), text: encUUIDText},
	pgtype.DateOID:        {json: quoted(encDateText), text: encDateText},
	pgtype.TimeOID:        {json: quoted(encTimeText), text: encTimeText},
	pgtype.TimestampOID:   {json: quoted(encTimestampText), text: encTimestampText},
	pgtype.TimestamptzOID: {json: quoted(encTimestamptzText), text: encTimestamptzText},
	pgtype.JSONOID:        {json: encRaw, text: encJSONTextForm},
	pgtype.JSONBOID:       {json: encJSONB, text: encJSONBTextForm},
}

// quoted wraps a text formatter whose output needs JSON string quotes but never
// escaping (dates, times, uuids: no quote, backslash, or control bytes).
func quoted(fn func(dst, src []byte) ([]byte, error)) func(dst, src []byte) ([]byte, error) {
	return func(dst, src []byte) ([]byte, error) {
		dst, err := fn(append(dst, '"'), src)
		if err != nil {
			return nil, err
		}
		return append(dst, '"'), nil
	}
}

func encRaw(dst, src []byte) ([]byte, error) { return append(dst, src...), nil }

// beInt16/32/64 read big-endian two's-complement values; the shift assembly
// compiles to a single load+byteswap.
func beInt16(b []byte) int16 {
	return int16(b[0])<<8 | int16(b[1])
}

func beInt32(b []byte) int32 {
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3])
}

func beInt64(b []byte) int64 {
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}

func encBool(dst, src []byte) ([]byte, error) {
	if len(src) != 1 {
		return nil, fmt.Errorf("bool value: %d bytes", len(src))
	}
	if src[0] != 0 {
		return append(dst, "true"...), nil
	}
	return append(dst, "false"...), nil
}

func encInt2(dst, src []byte) ([]byte, error) {
	if len(src) != 2 {
		return nil, fmt.Errorf("int2 value: %d bytes", len(src))
	}
	return strconv.AppendInt(dst, int64(beInt16(src)), 10), nil
}

func encInt4(dst, src []byte) ([]byte, error) {
	if len(src) != 4 {
		return nil, fmt.Errorf("int4 value: %d bytes", len(src))
	}
	return strconv.AppendInt(dst, int64(beInt32(src)), 10), nil
}

func encInt8(dst, src []byte) ([]byte, error) {
	if len(src) != 8 {
		return nil, fmt.Errorf("int8 value: %d bytes", len(src))
	}
	return strconv.AppendInt(dst, beInt64(src), 10), nil
}

// appendFloat renders a float shortest-round-trip. The rendering can differ
// from float8out (1.234567e+06 vs 1234567) but parses to the identical value.
// NaN/±Infinity have no JSON number form; to_jsonb emits them as strings.
func appendFloat(dst []byte, f float64, bits int, jsonForm bool) []byte {
	switch {
	case math.IsNaN(f):
		return appendSpecial(dst, "NaN", jsonForm)
	case math.IsInf(f, 1):
		return appendSpecial(dst, "Infinity", jsonForm)
	case math.IsInf(f, -1):
		return appendSpecial(dst, "-Infinity", jsonForm)
	}
	return strconv.AppendFloat(dst, f, 'g', -1, bits)
}

func appendSpecial(dst []byte, s string, quote bool) []byte {
	if quote {
		dst = append(dst, '"')
		dst = append(dst, s...)
		return append(dst, '"')
	}
	return append(dst, s...)
}

func float4From(src []byte) (float64, error) {
	if len(src) != 4 {
		return 0, fmt.Errorf("float4 value: %d bytes", len(src))
	}
	return float64(math.Float32frombits(binary.BigEndian.Uint32(src))), nil
}

func float8From(src []byte) (float64, error) {
	if len(src) != 8 {
		return 0, fmt.Errorf("float8 value: %d bytes", len(src))
	}
	return math.Float64frombits(binary.BigEndian.Uint64(src)), nil
}

func encFloat4JSON(dst, src []byte) ([]byte, error) {
	f, err := float4From(src)
	if err != nil {
		return nil, err
	}
	return appendFloat(dst, f, 32, true), nil
}

func encFloat4Text(dst, src []byte) ([]byte, error) {
	f, err := float4From(src)
	if err != nil {
		return nil, err
	}
	return appendFloat(dst, f, 32, false), nil
}

func encFloat8JSON(dst, src []byte) ([]byte, error) {
	f, err := float8From(src)
	if err != nil {
		return nil, err
	}
	return appendFloat(dst, f, 64, true), nil
}

func encFloat8Text(dst, src []byte) ([]byte, error) {
	f, err := float8From(src)
	if err != nil {
		return nil, err
	}
	return appendFloat(dst, f, 64, false), nil
}

// Binary numeric wire format: int16 ndigits, weight, sign, dscale, then
// ndigits base-10000 digit groups. Special sign values mark NaN/±Infinity.
const (
	numericNeg  = 0x4000
	numericNaN  = 0xC000
	numericPInf = 0xD000
	numericNInf = 0xF000
)

func encNumericJSON(dst, src []byte) ([]byte, error) { return appendNumeric(dst, src, true) }
func encNumericText(dst, src []byte) ([]byte, error) { return appendNumeric(dst, src, false) }

// appendNumeric renders the exact decimal text numeric_out produces, so the
// JSON number keeps full precision and scale (1.5000 stays 1.5000).
func appendNumeric(dst, src []byte, jsonForm bool) ([]byte, error) {
	if len(src) < 8 {
		return nil, fmt.Errorf("numeric value: %d bytes", len(src))
	}
	ndigits := int(binary.BigEndian.Uint16(src[0:2]))
	weight := int(beInt16(src[2:4]))
	sign := binary.BigEndian.Uint16(src[4:6])
	dscale := int(binary.BigEndian.Uint16(src[6:8]))

	switch sign {
	case numericNaN:
		return appendSpecial(dst, "NaN", jsonForm), nil
	case numericPInf:
		return appendSpecial(dst, "Infinity", jsonForm), nil
	case numericNInf:
		return appendSpecial(dst, "-Infinity", jsonForm), nil
	case numericNeg:
		dst = append(dst, '-')
	case 0:
	default:
		return nil, fmt.Errorf("numeric sign %#x", sign)
	}
	if len(src) != 8+2*ndigits {
		return nil, fmt.Errorf("numeric value: %d bytes for %d digits", len(src), ndigits)
	}
	digit := func(i int) int64 {
		if i < 0 || i >= ndigits {
			return 0
		}
		return int64(binary.BigEndian.Uint16(src[8+2*i:]))
	}

	// Integer part: the first group unpadded, the rest zero-padded to 4.
	if weight < 0 {
		dst = append(dst, '0')
	} else {
		dst = strconv.AppendInt(dst, digit(0), 10)
		for i := 1; i <= weight; i++ {
			dst = appendPadded(dst, digit(i), 4)
		}
	}
	// Fraction: exactly dscale digits from the groups after the decimal point.
	if dscale > 0 {
		dst = append(dst, '.')
		start := len(dst)
		for i := weight + 1; len(dst)-start < dscale; i++ {
			dst = appendPadded(dst, digit(i), 4)
		}
		dst = dst[:start+dscale]
	}
	return dst, nil
}

func encByteaJSON(dst, src []byte) ([]byte, error) {
	dst = append(dst, '"', '\\', '\\', 'x')
	dst = hex.AppendEncode(dst, src)
	return append(dst, '"'), nil
}

// encByteaText renders bytea as \x-prefixed hex, matching bytea_output=hex.
func encByteaText(dst, src []byte) ([]byte, error) {
	dst = append(dst, '\\', 'x')
	return hex.AppendEncode(dst, src), nil
}

func encUUIDText(dst, src []byte) ([]byte, error) {
	if len(src) != 16 {
		return nil, fmt.Errorf("uuid value: %d bytes", len(src))
	}
	dst = hex.AppendEncode(dst, src[0:4])
	dst = append(dst, '-')
	dst = hex.AppendEncode(dst, src[4:6])
	dst = append(dst, '-')
	dst = hex.AppendEncode(dst, src[6:8])
	dst = append(dst, '-')
	dst = hex.AppendEncode(dst, src[8:10])
	dst = append(dst, '-')
	return hex.AppendEncode(dst, src[10:16]), nil
}

// pgEpochDays is the Postgres date/timestamp epoch (2000-01-01) in days since
// the Unix epoch. Day arithmetic stays in days so the far end of the timestamp
// range cannot overflow int64 microseconds.
const pgEpochDays = 10957

const (
	usPerSecond = int64(1_000_000)
	usPerMinute = 60 * usPerSecond
	usPerHour   = 60 * usPerMinute
	usPerDay    = 24 * usPerHour
)

func encDateText(dst, src []byte) ([]byte, error) {
	if len(src) != 4 {
		return nil, fmt.Errorf("date value: %d bytes", len(src))
	}
	v := beInt32(src)
	switch v {
	case math.MaxInt32:
		return append(dst, "infinity"...), nil
	case math.MinInt32:
		return append(dst, "-infinity"...), nil
	}
	y, m, d := civilFromDays(int64(v) + pgEpochDays)
	if y <= 0 {
		return append(appendISODate(dst, 1-y, m, d), " BC"...), nil
	}
	return appendISODate(dst, y, m, d), nil
}

func encTimeText(dst, src []byte) ([]byte, error) {
	if len(src) != 8 {
		return nil, fmt.Errorf("time value: %d bytes", len(src))
	}
	return appendTimeOfDay(dst, beInt64(src)), nil
}

func encTimestampText(dst, src []byte) ([]byte, error) { return appendTimestamp(dst, src, false) }

// encTimestamptzText formats at UTC (+00:00). to_jsonb rendered the session
// TimeZone's offset instead — a different spelling of the same instant, which
// the sink's timestamptz cast resolves identically.
func encTimestamptzText(dst, src []byte) ([]byte, error) { return appendTimestamp(dst, src, true) }

// appendTimestamp renders binary micros-since-2000 as ISO 8601 with a T
// separator — the shape to_jsonb uses, not the space-separated timestamp_out.
func appendTimestamp(dst, src []byte, utcOffset bool) ([]byte, error) {
	if len(src) != 8 {
		return nil, fmt.Errorf("timestamp value: %d bytes", len(src))
	}
	us := beInt64(src)
	switch us {
	case math.MaxInt64:
		return append(dst, "infinity"...), nil
	case math.MinInt64:
		return append(dst, "-infinity"...), nil
	}
	days := floorDiv(us, usPerDay)
	y, m, d := civilFromDays(days + pgEpochDays)
	bc := y <= 0
	if bc {
		y = 1 - y
	}
	dst = appendISODate(dst, y, m, d)
	dst = append(dst, 'T')
	dst = appendTimeOfDay(dst, us-days*usPerDay)
	if utcOffset {
		dst = append(dst, "+00:00"...)
	}
	if bc {
		dst = append(dst, " BC"...)
	}
	return dst, nil
}

// appendTimeOfDay renders microseconds-since-midnight as HH:MM:SS with the
// fractional part trimmed of trailing zeros, matching timestamp_out.
func appendTimeOfDay(dst []byte, us int64) []byte {
	dst = appendPadded(dst, us/usPerHour, 2)
	dst = append(dst, ':')
	dst = appendPadded(dst, us%usPerHour/usPerMinute, 2)
	dst = append(dst, ':')
	dst = appendPadded(dst, us%usPerMinute/usPerSecond, 2)
	if frac := us % usPerSecond; frac > 0 {
		dst = append(dst, '.')
		dst = appendPadded(dst, frac, 6)
		for dst[len(dst)-1] == '0' {
			dst = dst[:len(dst)-1]
		}
	}
	return dst
}

func appendISODate(dst []byte, y int64, m, d int) []byte {
	dst = appendPadded(dst, y, 4)
	dst = append(dst, '-')
	dst = appendPadded(dst, int64(m), 2)
	dst = append(dst, '-')
	return appendPadded(dst, int64(d), 2)
}

// appendPadded appends v left-padded with zeros to width digits.
func appendPadded(dst []byte, v int64, width int) []byte {
	var buf [20]byte
	b := strconv.AppendInt(buf[:0], v, 10)
	for i := len(b); i < width; i++ {
		dst = append(dst, '0')
	}
	return append(dst, b...)
}

// floorDiv is integer division rounding toward negative infinity (b > 0).
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// civilFromDays converts days since 1970-01-01 to a proleptic Gregorian date
// (Hinnant's civil_from_days). Year <= 0 means BC (0 → 1 BC).
func civilFromDays(days int64) (y int64, m, d int) {
	z := days + 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = int(doy - (153*mp+2)/5 + 1)
	if mp < 10 {
		m = int(mp) + 3
	} else {
		m = int(mp) - 9
	}
	y = yoe + era*400
	if m <= 2 {
		y++
	}
	return y, m, d
}

func encJSONB(dst, src []byte) ([]byte, error) {
	if len(src) == 0 || src[0] != 1 {
		return nil, fmt.Errorf("jsonb version byte")
	}
	return append(dst, src[1:]...), nil
}

func encJSONBTextForm(dst, src []byte) ([]byte, error) {
	if len(src) == 0 || src[0] != 1 {
		return nil, fmt.Errorf("jsonb version byte")
	}
	return encJSONTextForm(dst, src[1:])
}

// encJSONTextForm renders a JSON value's ->> text form: strings unescaped,
// everything else verbatim. Cold path — reached only when an id or cursor
// column has no native formatter.
func encJSONTextForm(dst, src []byte) ([]byte, error) {
	if len(src) == 0 || src[0] != '"' {
		return append(dst, src...), nil
	}
	var s string
	if err := json.Unmarshal(src, &s); err != nil {
		return nil, fmt.Errorf("fallback text form: %w", err)
	}
	return append(dst, s...), nil
}

// appendTID renders a 6-byte tid as "(block,offset)", matching tid_out.
func appendTID(dst, src []byte) ([]byte, error) {
	if len(src) != 6 {
		return nil, fmt.Errorf("tid value: %d bytes", len(src))
	}
	dst = append(dst, '(')
	dst = strconv.AppendUint(dst, uint64(binary.BigEndian.Uint32(src[0:4])), 10)
	dst = append(dst, ',')
	dst = strconv.AppendUint(dst, uint64(binary.BigEndian.Uint16(src[4:6])), 10)
	return append(dst, ')'), nil
}

const hexDigits = "0123456789abcdef"

// encJSONString escapes a text value as a JSON string: named escapes for the
// common controls, \u00XX for the rest, UTF-8 passed through — the same
// escaping semantics as to_jsonb's escape_json.
func encJSONString(dst, src []byte) ([]byte, error) {
	dst = append(dst, '"')
	start := 0
	for i := range len(src) {
		b := src[i]
		if b >= 0x20 && b != '"' && b != '\\' {
			continue
		}
		dst = append(dst, src[start:i]...)
		switch b {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, '\\', 'u', '0', '0', hexDigits[b>>4], hexDigits[b&0xF])
		}
		start = i + 1
	}
	dst = append(dst, src[start:]...)
	return append(dst, '"'), nil
}
