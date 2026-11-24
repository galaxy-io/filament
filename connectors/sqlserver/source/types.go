package sqlserver

// Every SQL Server type this source understands, in one place: the logical type
// a catalog column maps to, the expression that selects it, the decoder that
// turns its TDS value bytes into a row, and the canonical text a key value is
// checkpointed as.
//
// Values arrive through go-mssqldb's RawRows as the bytes the server sent, so a
// decoder is chosen once per result column from its TDS type and then runs per
// value without a driver.Value, a string, a math/big decimal or a time.Time in
// between. CLR types, sql_variant, json and vector are converted by the server
// in the select list, since their wire forms are not portable values.

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/apache/arrow-go/v18/arrow/decimal128"
	mssql "github.com/microsoft/go-mssqldb"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// column is one catalog column of a table.
type column struct {
	name      string
	typeName  string // base system type, or the CLR type name
	maxLength int    // bytes; -1 for (max)
	precision int
	scale     int
	nullable  bool
	pkOrder   int    // 1-based position in the primary key; 0 = not a key column
	collation string // the column's collation; empty for a non-character type
}

// decl renders the column's declared type, the form CAST targets and
// SchemaField.Native use.
func (c column) decl() string {
	switch c.typeName {
	case "decimal", "numeric":
		return fmt.Sprintf("%s(%d,%d)", c.typeName, c.precision, c.scale)
	case "char", "varchar", "binary", "varbinary":
		if c.maxLength < 0 {
			return c.typeName + "(max)"
		}
		return fmt.Sprintf("%s(%d)", c.typeName, c.maxLength)
	case "nchar", "nvarchar":
		if c.maxLength < 0 {
			return c.typeName + "(max)"
		}
		return fmt.Sprintf("%s(%d)", c.typeName, c.maxLength/2)
	case "datetime2", "datetimeoffset", "time":
		return fmt.Sprintf("%s(%d)", c.typeName, c.scale)
	default:
		return c.typeName
	}
}

// keyDecl is the declared type a checkpoint records for a key column. A text
// key also names its collation, which orders it: a changed collation is then
// a changed key, and a plan laid out under the old order starts over. A
// resume key travels as a Unicode parameter, and converting it to
// non-Unicode text under any other collation's code page could change its
// characters, so its cast applies the column's collation first.
func (c column) keyDecl() string {
	switch c.typeName {
	case "char", "varchar", "nchar", "nvarchar":
		if c.collation != "" {
			return c.decl() + collatePrefix + c.collation
		}
	}
	return c.decl()
}

// collatePrefix separates a key declaration's type from its collation.
const collatePrefix = " COLLATE "

// sqlType is how a column type maps into the row model and the select list.
type sqlType struct {
	logical          rowmodel.LogicalType
	precision, scale int
	// selectExpr wraps the column identifier when the server must convert the
	// value; nil selects the column as it is.
	selectExpr func(ident string) string
}

// typeFor classifies a catalog column.
func typeFor(c column) sqlType {
	switch c.typeName {
	case "bit":
		return sqlType{logical: rowmodel.LogicalBool}
	case "tinyint", "smallint":
		return sqlType{logical: rowmodel.LogicalInt16}
	case "int":
		return sqlType{logical: rowmodel.LogicalInt32}
	case "bigint":
		return sqlType{logical: rowmodel.LogicalInt64}
	case "real": // float(1..24) is catalogued as real
		return sqlType{logical: rowmodel.LogicalFloat32}
	case "float":
		return sqlType{logical: rowmodel.LogicalFloat64}
	case "decimal", "numeric":
		return sqlType{logical: rowmodel.LogicalDecimal, precision: c.precision, scale: c.scale}
	case "money":
		return sqlType{logical: rowmodel.LogicalDecimal, precision: 19, scale: moneyScale}
	case "smallmoney":
		return sqlType{logical: rowmodel.LogicalDecimal, precision: 10, scale: moneyScale}
	case "date":
		return sqlType{logical: rowmodel.LogicalDate}
	case "time":
		return sqlType{logical: rowmodel.LogicalTime}
	case "datetime", "smalldatetime", "datetime2":
		return sqlType{logical: rowmodel.LogicalTimestamp}
	case "datetimeoffset":
		return sqlType{logical: rowmodel.LogicalTimestampTZ}
	case "char", "varchar", "text", "nchar", "nvarchar", "ntext", "xml":
		return sqlType{logical: rowmodel.LogicalString}
	case "binary", "varbinary", "image", "timestamp", "rowversion":
		return sqlType{logical: rowmodel.LogicalBytes}
	case "uniqueidentifier":
		return sqlType{logical: rowmodel.LogicalUUID}
	case "hierarchyid":
		return sqlType{logical: rowmodel.LogicalString, selectExpr: func(id string) string { return id + ".ToString()" }}
	case "geometry", "geography":
		// Well-known binary, the portable form every spatial engine reads,
		// with Z and M kept. WKB has no SRID; the docs say so.
		return sqlType{logical: rowmodel.LogicalBytes, selectExpr: func(id string) string { return id + ".AsBinaryZM()" }}
	case "json":
		return sqlType{logical: rowmodel.LogicalJSON, selectExpr: castNVarcharMax}
	default: // sql_variant, vector and anything newer travel as their text form
		return sqlType{logical: rowmodel.LogicalString, selectExpr: castNVarcharMax}
	}
}

func castNVarcharMax(ident string) string { return "CAST(" + ident + " AS nvarchar(max))" }

// decodeFunc appends one non-null value. r carries per-query scratch space.
type decodeFunc func(r *rowReader, w arrowbatch.RowWriter, v []byte) error

// decoderFor picks the decoder for a result column that lands in field f. It
// fails when the server sends a type the field's logical type cannot hold,
// which means the table changed between the catalog read and the query.
func decoderFor(col *mssql.RawColumn, f rowmodel.Field) (decodeFunc, error) {
	mismatch := func() (decodeFunc, error) {
		return nil, fmt.Errorf("column %q: server sent TDS type %#x for a %s field", f.Name, uint8(col.Type), f.Logical)
	}
	switch f.Logical {
	case rowmodel.LogicalBool:
		if col.Type != mssql.TDSBit && col.Type != mssql.TDSBitN {
			return mismatch()
		}
		return decBool, nil
	case rowmodel.LogicalInt16, rowmodel.LogicalInt32, rowmodel.LogicalInt64:
		switch col.Type {
		case mssql.TDSInt1, mssql.TDSInt2, mssql.TDSInt4, mssql.TDSInt8, mssql.TDSIntN:
			return intDecoder(f.Logical), nil
		}
		return mismatch()
	case rowmodel.LogicalFloat32, rowmodel.LogicalFloat64:
		switch col.Type {
		case mssql.TDSFlt4, mssql.TDSFlt8, mssql.TDSFltN:
			return floatDecoder(f.Logical), nil
		}
		return mismatch()
	case rowmodel.LogicalDecimal:
		switch col.Type {
		case mssql.TDSDecimalN, mssql.TDSNumericN, mssql.TDSDecimal, mssql.TDSNumeric:
			// A column widened since the catalog read can hold values the
			// field cannot.
			if int(col.Scale) != f.Scale || int(col.Precision) > f.Precision {
				return mismatch()
			}
			return decDecimal, nil
		case mssql.TDSMoney, mssql.TDSMoney4, mssql.TDSMoneyN:
			if f.Scale != moneyScale || moneyPrecision(col) > f.Precision {
				return mismatch()
			}
			return decMoney, nil
		}
		return mismatch()
	case rowmodel.LogicalDate:
		if col.Type != mssql.TDSDateN {
			return mismatch()
		}
		return decDate, nil
	case rowmodel.LogicalTime:
		if col.Type != mssql.TDSTimeN {
			return mismatch()
		}
		return timeDecoder(int(col.Scale)), nil
	case rowmodel.LogicalTimestamp:
		switch col.Type {
		case mssql.TDSDateTime2N:
			return datetime2Decoder(int(col.Scale)), nil
		case mssql.TDSDateTime, mssql.TDSDateTim4, mssql.TDSDateTimeN:
			return decDatetime, nil
		}
		return mismatch()
	case rowmodel.LogicalTimestampTZ:
		if col.Type != mssql.TDSDateTimeOffsetN {
			return mismatch()
		}
		return datetimeoffsetDecoder(int(col.Scale)), nil
	case rowmodel.LogicalUUID:
		if col.Type != mssql.TDSGuid {
			return mismatch()
		}
		return decGUID, nil
	default: // bytes, string and json
		if dec := varLenDecoder(col, f.Logical); dec != nil {
			return dec, nil
		}
		return mismatch()
	}
}

// varLenDecoder picks the decoder for a bytes, string or json field, nil when
// the TDS type cannot fill it.
func varLenDecoder(col *mssql.RawColumn, l rowmodel.LogicalType) decodeFunc {
	if l == rowmodel.LogicalBytes {
		switch col.Type {
		case mssql.TDSBinary, mssql.TDSVarBinary, mssql.TDSBigBinary, mssql.TDSBigVarBin, mssql.TDSImage, mssql.TDSUdt:
			return decBytes
		}
		return nil
	}
	switch col.Type {
	case mssql.TDSNChar, mssql.TDSNVarChar, mssql.TDSNText, mssql.TDSXml:
		return decUTF16
	case mssql.TDSChar, mssql.TDSVarChar, mssql.TDSBigChar, mssql.TDSBigVarChar, mssql.TDSText:
		return charDecoder(col)
	}
	return nil
}

func decBool(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	if len(v) != 1 {
		return fmt.Errorf("bit of %d bytes", len(v))
	}
	w.Bool(v[0] != 0)
	return nil
}

// readInt reads a 1, 2, 4 or 8 byte little-endian integer. A one-byte integer
// is tinyint, which SQL Server keeps unsigned.
func readInt(v []byte) (int64, error) {
	switch len(v) {
	case 1:
		return int64(v[0]), nil
	case 2:
		return int64(int16(binary.LittleEndian.Uint16(v))), nil //nolint:gosec // two's complement
	case 4:
		return int64(int32(binary.LittleEndian.Uint32(v))), nil //nolint:gosec // two's complement
	case 8:
		return int64(binary.LittleEndian.Uint64(v)), nil //nolint:gosec // two's complement
	}
	return 0, fmt.Errorf("integer of %d bytes", len(v))
}

func intDecoder(l rowmodel.LogicalType) decodeFunc {
	switch l {
	case rowmodel.LogicalInt16:
		return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
			n, err := readInt(v)
			if err != nil {
				return err
			}
			if n < math.MinInt16 || n > math.MaxInt16 {
				return errIntRange(n, l)
			}
			w.Int16(int16(n))
			return nil
		}
	case rowmodel.LogicalInt32:
		return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
			n, err := readInt(v)
			if err != nil {
				return err
			}
			if n < math.MinInt32 || n > math.MaxInt32 {
				return errIntRange(n, l)
			}
			w.Int32(int32(n))
			return nil
		}
	default:
		return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
			n, err := readInt(v)
			if err != nil {
				return err
			}
			w.Int64(n)
			return nil
		}
	}
}

// errIntRange reports an integer wider than the field the catalog declared,
// which happens when the column is widened after the run read the catalog.
// Narrowing it would write a different number.
func errIntRange(n int64, l rowmodel.LogicalType) error {
	return fmt.Errorf("value %d does not fit %s; the column was widened since the catalog was read", n, l)
}

func readFloat(v []byte) (float64, error) {
	switch len(v) {
	case 4:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(v))), nil
	case 8:
		return math.Float64frombits(binary.LittleEndian.Uint64(v)), nil
	}
	return 0, fmt.Errorf("float of %d bytes", len(v))
}

func floatDecoder(l rowmodel.LogicalType) decodeFunc {
	if l == rowmodel.LogicalFloat32 {
		return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
			f, err := readFloat(v)
			if err != nil {
				return err
			}
			// A float read into a real field when the column was widened
			// after the catalog read: a value real cannot hold would become
			// infinity.
			n := float32(f)
			if !math.IsInf(f, 0) && math.IsInf(float64(n), 0) {
				return fmt.Errorf("value %g does not fit %s; the column was widened since the catalog was read", f, l)
			}
			w.Float32(n)
			return nil
		}
	}
	return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
		f, err := readFloat(v)
		if err != nil {
			return err
		}
		w.Float64(f)
		return nil
	}
}

// readDecimal reads a decimal or numeric value: a sign byte (1 = positive)
// and a little-endian magnitude of 4, 8, 12 or 16 bytes.
func readDecimal(v []byte) (decimal128.Num, error) {
	if len(v) < 5 || len(v) > 17 {
		return decimal128.Num{}, fmt.Errorf("decimal of %d bytes", len(v))
	}
	var mag [16]byte
	copy(mag[:], v[1:])
	n := decimal128.New(int64(binary.LittleEndian.Uint64(mag[8:])), binary.LittleEndian.Uint64(mag[:])) //nolint:gosec // 128-bit magnitude
	if v[0] == 0 {
		n = n.Negate()
	}
	return n, nil
}

func decDecimal(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	n, err := readDecimal(v)
	if err != nil {
		return err
	}
	w.Decimal(n)
	return nil
}

// moneyScale is the fixed number of fractional digits of money and
// smallmoney.
const moneyScale = 4

// moneyPrecision is the precision of a money result column, which TDS does
// not send: money is 8 bytes and precision 19, smallmoney 4 bytes and 10.
func moneyPrecision(col *mssql.RawColumn) int {
	if col.Type == mssql.TDSMoney4 || (col.Type == mssql.TDSMoneyN && col.Size == 4) {
		return 10
	}
	return 19
}

// readMoney reads money (the high 32 bits, then the low 32 bits) or
// smallmoney (one int32), both in units of 1/10000.
func readMoney(v []byte) (int64, error) {
	switch len(v) {
	case 4:
		return int64(int32(binary.LittleEndian.Uint32(v))), nil //nolint:gosec // two's complement
	case 8:
		hi := int64(int32(binary.LittleEndian.Uint32(v))) //nolint:gosec // two's complement
		lo := int64(binary.LittleEndian.Uint32(v[4:]))
		return hi<<32 | lo, nil
	}
	return 0, fmt.Errorf("money of %d bytes", len(v))
}

func decMoney(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	n, err := readMoney(v)
	if err != nil {
		return err
	}
	w.Decimal(decimal128.FromI64(n))
	return nil
}

// Days between 0001-01-01, the TDS date epoch, and 1970-01-01, and between
// 1900-01-01, the datetime epoch, and 1970-01-01.
const (
	tdsEpochDays      = 719162
	datetimeEpochDays = 25567
	microsPerDay      = 86_400_000_000
)

func readDate(v []byte) (int64, error) {
	if len(v) != 3 {
		return 0, fmt.Errorf("date of %d bytes", len(v))
	}
	return int64(v[0]) | int64(v[1])<<8 | int64(v[2])<<16 - tdsEpochDays, nil
}

func decDate(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	d, err := readDate(v)
	if err != nil {
		return err
	}
	w.Date(int32(d)) //nolint:gosec // 0001..9999
	return nil
}

// readTimeUnits reads a time of day as 10^-scale second units from 3 to 5
// little-endian bytes.
func readTimeUnits(v []byte) (int64, error) {
	if len(v) < 3 || len(v) > 5 {
		return 0, fmt.Errorf("time of %d bytes", len(v))
	}
	var u int64
	for i := len(v) - 1; i >= 0; i-- {
		u = u<<8 | int64(v[i])
	}
	return u, nil
}

// unitsToMicros converts 10^-scale second units to microseconds, truncating
// the 100ns digit of a scale-7 value: the row model's times are microseconds.
func unitsToMicros(u int64, scale int) int64 {
	if scale <= 6 {
		return u * pow10[6-scale]
	}
	return u / pow10[scale-6]
}

var pow10 = [...]int64{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000}

func timeDecoder(scale int) decodeFunc {
	return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
		u, err := readTimeUnits(v)
		if err != nil {
			return err
		}
		w.Time(unitsToMicros(u, scale))
		return nil
	}
}

// readDatetime2 splits a datetime2 value into days since the Unix epoch and
// time units: the time bytes come first, then a 3-byte date.
func readDatetime2(v []byte) (days, units int64, err error) {
	if len(v) < 6 {
		return 0, 0, fmt.Errorf("datetime2 of %d bytes", len(v))
	}
	if units, err = readTimeUnits(v[:len(v)-3]); err != nil {
		return 0, 0, err
	}
	days, err = readDate(v[len(v)-3:])
	return days, units, err
}

func datetime2Decoder(scale int) decodeFunc {
	return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
		days, units, err := readDatetime2(v)
		if err != nil {
			return err
		}
		w.Timestamp(days*microsPerDay + unitsToMicros(units, scale))
		return nil
	}
}

// datetimeoffsetDecoder reads a datetimeoffset: a datetime2 in UTC followed by
// the offset in minutes, which the instant does not need.
func datetimeoffsetDecoder(scale int) decodeFunc {
	return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
		if len(v) < 8 {
			return fmt.Errorf("datetimeoffset of %d bytes", len(v))
		}
		days, units, err := readDatetime2(v[:len(v)-2])
		if err != nil {
			return err
		}
		w.Timestamp(days*microsPerDay + unitsToMicros(units, scale))
		return nil
	}
}

// readDatetime reads datetime (int32 days since 1900, uint32 1/300 second
// ticks) or smalldatetime (uint16 days since 1900, uint16 minutes) as
// microseconds since the Unix epoch.
func readDatetime(v []byte) (int64, error) {
	switch len(v) {
	case 4:
		days := int64(binary.LittleEndian.Uint16(v)) - datetimeEpochDays
		minutes := int64(binary.LittleEndian.Uint16(v[2:]))
		return days*microsPerDay + minutes*60_000_000, nil
	case 8:
		days := int64(int32(binary.LittleEndian.Uint32(v))) - datetimeEpochDays //nolint:gosec // two's complement
		ticks := int64(binary.LittleEndian.Uint32(v[4:]))
		// A tick is 10/3 ms; round to the nearest microsecond.
		return days*microsPerDay + (ticks*10_000+1)/3, nil
	}
	return 0, fmt.Errorf("datetime of %d bytes", len(v))
}

func decDatetime(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	us, err := readDatetime(v)
	if err != nil {
		return err
	}
	w.Timestamp(us)
	return nil
}

func decGUID(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	if len(v) != 16 {
		return fmt.Errorf("uniqueidentifier of %d bytes", len(v))
	}
	var buf [36]byte
	w.StringBytes(appendGUID(buf[:0], v))
	return nil
}

// appendGUID renders a uniqueidentifier the way SQL Server displays it. The
// first three groups are stored little-endian.
func appendGUID(dst, v []byte) []byte {
	const hex = "0123456789ABCDEF"
	order := [16]int{3, 2, 1, 0, 5, 4, 7, 6, 8, 9, 10, 11, 12, 13, 14, 15}
	for i, j := range order {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			dst = append(dst, '-')
		}
		dst = append(dst, hex[v[j]>>4], hex[v[j]&0x0f])
	}
	return dst
}

func decBytes(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
	w.Bytes(v)
	return nil
}

// decUTF16 converts UCS-2/UTF-16LE text into the reader's scratch buffer.
func decUTF16(r *rowReader, w arrowbatch.RowWriter, v []byte) error {
	r.utf8 = appendUTF16(r.utf8[:0], v)
	w.StringBytes(r.utf8)
	return nil
}

func appendUTF16(dst, v []byte) []byte {
	for i := 0; i+1 < len(v); i += 2 {
		c := rune(binary.LittleEndian.Uint16(v[i:]))
		if c < utf8.RuneSelf {
			dst = append(dst, byte(c))
			continue
		}
		if utf16.IsSurrogate(c) && i+3 < len(v) {
			c = utf16.DecodeRune(c, rune(binary.LittleEndian.Uint16(v[i+2:])))
			if c != utf8.RuneError {
				i += 2
			}
		}
		dst = utf8.AppendRune(dst, c)
	}
	return dst
}

// hasLoneSurrogate reports a UTF-16LE value holding a surrogate that is not
// part of a pair.
func hasLoneSurrogate(v []byte) bool {
	for i := 0; i+1 < len(v); i += 2 {
		c := rune(binary.LittleEndian.Uint16(v[i:]))
		switch {
		case c >= 0xd800 && c < 0xdc00: // high: a low must follow
			if i+3 >= len(v) {
				return true
			}
			if next := rune(binary.LittleEndian.Uint16(v[i+2:])); next < 0xdc00 || next > 0xdfff {
				return true
			}
			i += 2
		case c >= 0xdc00 && c <= 0xdfff: // low without a high
			return true
		}
	}
	return false
}

// charDecoder decodes non-Unicode text. ASCII is the same bytes in every code
// page SQL Server uses and in UTF-8, so it is appended as it is; anything else
// is decoded by the column's collation.
func charDecoder(col *mssql.RawColumn) decodeFunc {
	return func(_ *rowReader, w arrowbatch.RowWriter, v []byte) error {
		if isASCII(v) {
			w.StringBytes(v)
			return nil
		}
		s, err := col.DecodeString(v)
		if err != nil {
			return err
		}
		w.String(s)
		return nil
	}
}

func isASCII(b []byte) bool {
	for len(b) >= 8 {
		if binary.LittleEndian.Uint64(b)&0x8080808080808080 != 0 {
			return false
		}
		b = b[8:]
	}
	for _, c := range b {
		if c >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// ── Checkpoint keys ─────────────────────────────────────────────────────────

const binaryCheckpointPrefix = "b64:"

// keyFormatter renders one key value as its durable checkpoint text: a form
// that CAST back to the column's declared type yields the same value, so a
// resumed comparison is exact.
type keyFormatter func(v []byte) (string, error)

// keyFormatterFor picks the checkpoint text form for a key result column.
func keyFormatterFor(col *mssql.RawColumn) (keyFormatter, error) {
	switch col.Type {
	case mssql.TDSInt1, mssql.TDSInt2, mssql.TDSInt4, mssql.TDSInt8, mssql.TDSIntN:
		return func(v []byte) (string, error) {
			n, err := readInt(v)
			return strconv.FormatInt(n, 10), err
		}, nil
	case mssql.TDSBit, mssql.TDSBitN:
		return func(v []byte) (string, error) {
			if len(v) == 1 && v[0] != 0 {
				return "1", nil
			}
			return "0", nil
		}, nil
	case mssql.TDSDecimalN, mssql.TDSNumericN, mssql.TDSDecimal, mssql.TDSNumeric:
		scale := int32(col.Scale)
		return func(v []byte) (string, error) {
			n, err := readDecimal(v)
			return n.ToString(scale), err
		}, nil
	case mssql.TDSMoney, mssql.TDSMoney4, mssql.TDSMoneyN:
		return func(v []byte) (string, error) {
			n, err := readMoney(v)
			return decimal128.FromI64(n).ToString(4), err
		}, nil
	case mssql.TDSFlt4, mssql.TDSFlt8, mssql.TDSFltN:
		return func(v []byte) (string, error) {
			f, err := readFloat(v)
			return strconv.FormatFloat(f, 'g', -1, 8*len(v)), err
		}, nil
	case mssql.TDSDateN:
		return func(v []byte) (string, error) {
			d, err := readDate(v)
			return formatDate(d), err
		}, nil
	case mssql.TDSTimeN:
		scale := int(col.Scale)
		return func(v []byte) (string, error) {
			u, err := readTimeUnits(v)
			return formatTimeOfDay(u, scale), err
		}, nil
	case mssql.TDSDateTime2N:
		scale := int(col.Scale)
		return func(v []byte) (string, error) {
			days, units, err := readDatetime2(v)
			return formatDate(days) + "T" + formatTimeOfDay(units, scale), err
		}, nil
	case mssql.TDSDateTimeOffsetN:
		scale := int(col.Scale)
		return func(v []byte) (string, error) {
			if len(v) < 8 {
				return "", fmt.Errorf("datetimeoffset of %d bytes", len(v))
			}
			days, units, err := readDatetime2(v[:len(v)-2])
			return formatDate(days) + "T" + formatTimeOfDay(units, scale) + "Z", err
		}, nil
	case mssql.TDSDateTime, mssql.TDSDateTim4, mssql.TDSDateTimeN:
		return formatDatetimeKey, nil
	case mssql.TDSGuid:
		return func(v []byte) (string, error) {
			if len(v) != 16 {
				return "", fmt.Errorf("uniqueidentifier of %d bytes", len(v))
			}
			return string(appendGUID(nil, v)), nil
		}, nil
	case mssql.TDSNChar, mssql.TDSNVarChar:
		return func(v []byte) (string, error) {
			if hasLoneSurrogate(v) {
				// Its text would hold U+FFFD instead, which a resume compares
				// as a different key and so could skip rows.
				return "", errors.New("key holds an unpaired UTF-16 surrogate, which a checkpoint cannot restore")
			}
			return string(appendUTF16(nil, v)), nil
		}, nil
	case mssql.TDSChar, mssql.TDSVarChar, mssql.TDSBigChar, mssql.TDSBigVarChar:
		return func(v []byte) (string, error) {
			if isASCII(v) {
				return string(v), nil
			}
			return col.DecodeString(v)
		}, nil
	case mssql.TDSBinary, mssql.TDSVarBinary, mssql.TDSBigBinary, mssql.TDSBigVarBin:
		return func(v []byte) (string, error) {
			return binaryCheckpointPrefix + base64.RawStdEncoding.EncodeToString(v), nil
		}, nil
	}
	return nil, fmt.Errorf("column %q of TDS type %#x cannot be a checkpoint key", col.Name, uint8(col.Type))
}

// formatDatetimeKey renders datetime to the millisecond and smalldatetime to
// the minute. Both cast back to the same tick: datetime values are multiples of
// 1/300 s, and the nearest millisecond rounds to the same one.
func formatDatetimeKey(v []byte) (string, error) {
	us, err := readDatetime(v)
	if err != nil {
		return "", err
	}
	days := floorDiv(us, microsPerDay)
	tod := us - days*microsPerDay
	ms := (tod + 500) / 1000
	if ms == 86_400_000 { // a tick rounding up past midnight is not stored by SQL Server
		ms--
	}
	return formatDate(days) + "T" + formatTimeOfDay(ms, 3), nil
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// formatDate renders days since the Unix epoch as YYYY-MM-DD (proleptic
// Gregorian, years 1 to 9999).
func formatDate(days int64) string {
	y, m, d := civil(days)
	var b [10]byte
	put4(b[0:], y)
	b[4] = '-'
	put2(b[5:], m)
	b[7] = '-'
	put2(b[8:], d)
	return string(b[:])
}

// civil converts days since 1970-01-01 to a calendar date (Howard Hinnant's
// algorithm).
func civil(days int64) (year, month, day int) {
	z := days + 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return int(y), int(m), int(d)
}

// formatTimeOfDay renders 10^-scale second units as HH:MM:SS[.fraction] with
// exactly scale fractional digits.
func formatTimeOfDay(units int64, scale int) string {
	secs := units / pow10[scale]
	frac := units % pow10[scale]
	var b strings.Builder
	var buf [2]byte
	put2(buf[:], int(secs/3600))
	b.Write(buf[:])
	b.WriteByte(':')
	put2(buf[:], int(secs/60%60))
	b.Write(buf[:])
	b.WriteByte(':')
	put2(buf[:], int(secs%60))
	b.Write(buf[:])
	if scale > 0 {
		b.WriteByte('.')
		s := strconv.FormatInt(frac, 10)
		b.WriteString(strings.Repeat("0", scale-len(s)))
		b.WriteString(s)
	}
	return b.String()
}

func put2(b []byte, n int) { b[0], b[1] = byte('0'+n/10%10), byte('0'+n%10) }

func put4(b []byte, n int) {
	put2(b[0:], n/100)
	put2(b[2:], n%100)
}
