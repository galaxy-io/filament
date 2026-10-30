package arrowtext

import (
	"math/bits"
	"strconv"

	"github.com/apache/arrow-go/v18/arrow/decimal128"
)

// AppendDecimal appends the decimal text of v at scale ("-12.30"), without
// going through strings or big.Int. Scales outside [-MaxScale, MaxScale] use
// exact exponent notation to keep the output bounded.
func AppendDecimal(dst []byte, v decimal128.Num, scale int) []byte {
	if v.Sign() < 0 {
		dst = append(dst, '-')
		v = v.Negate()
	}
	var buf [40]byte
	digits := DecimalDigits(buf[:], uint64(v.HighBits()), v.LowBits()) //nolint:gosec // magnitude, sign handled above
	if scale < -decimal128.MaxScale || scale > decimal128.MaxScale {
		if len(digits) == 0 {
			return append(dst, '0')
		}
		dst = append(dst, digits...)
		dst = append(dst, 'e')
		if scale > 0 {
			dst = append(dst, '-')
			return strconv.AppendInt(dst, int64(scale), 10)
		}
		// Drop the formatted sign instead of negating the minimum int.
		var exponent [21]byte
		text := strconv.AppendInt(exponent[:0], int64(scale), 10)
		return append(dst, text[1:]...)
	}
	if scale < 0 {
		if len(digits) == 0 {
			return append(dst, '0')
		}
		dst = append(dst, digits...)
		for ; scale < 0; scale++ {
			dst = append(dst, '0')
		}
		return dst
	}
	if len(digits) <= scale {
		dst = append(dst, '0')
		if scale > 0 {
			dst = append(dst, '.')
			for i := len(digits); i < scale; i++ {
				dst = append(dst, '0')
			}
			dst = append(dst, digits...)
		}
		return dst
	}
	dst = append(dst, digits[:len(digits)-scale]...)
	if scale > 0 {
		dst = append(dst, '.')
		dst = append(dst, digits[len(digits)-scale:]...)
	}
	return dst
}

// DecimalDigits renders the unsigned 128-bit value hi:lo as decimal digits into
// buf (len >= 39) and returns them, without leading zeros; zero renders as no
// digits.
func DecimalDigits(buf []byte, hi, lo uint64) []byte {
	const chunk = 10_000_000_000_000_000_000 // 1e19, the largest power of ten in uint64
	i := len(buf)
	for hi != 0 {
		// Divide hi:lo by 1e19: the high word first, then the remainder folded into
		// the low word.
		qhi, r := hi/chunk, hi%chunk
		qlo, rem := bits.Div64(r, lo, chunk)
		hi, lo = qhi, qlo
		for range 19 {
			i--
			buf[i] = byte('0' + rem%10)
			rem /= 10
		}
	}
	for lo != 0 {
		i--
		buf[i] = byte('0' + lo%10)
		lo /= 10
	}
	return buf[i:]
}
