package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
	"github.com/apache/arrow-go/v18/arrow/scalar"
)

// defaultDateLayout is the layout ToDate assumes when given none.
const defaultDateLayout = "2006-01-02"

// ToDate parses every value of a utf8 column as a date using a Go time
// layout, given as an optional second argument. A value that does not parse
// fails the whole batch rather than becoming null.
func ToDate(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	ad, ok := args[0].(*compute.ArrayDatum)
	if !ok || ad.Value.DataType().ID() != arrow.STRING {
		return nil, fmt.Errorf("to_date: expected utf8 array, got %s", args[0])
	}
	layout := defaultDateLayout
	if len(args) > 1 {
		layout = args[1].(*compute.ScalarDatum).Value.(*scalar.String).String()
	}
	src := array.NewStringData(ad.Value)
	defer src.Release()

	b := array.NewDate32Builder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(src.Len())

	for i := range src.Len() {
		if src.IsNull(i) {
			b.AppendNull()
			continue
		}
		t, err := time.Parse(layout, src.Value(i))
		if err != nil {
			return nil, fmt.Errorf("to_date: row %d: %w", i, err)
		}
		b.UnsafeAppend(arrow.Date32FromTime(t))
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// Year returns the calendar year of each value in a date or timestamp
// column, as int64.
func Year(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return datePart(ctx, args[0], func(t time.Time) int64 { return int64(t.Year()) })
}

// Month returns the calendar month of each value, 1 through 12.
func Month(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return datePart(ctx, args[0], func(t time.Time) int64 { return int64(t.Month()) })
}

// Day returns the day of the month of each value, 1 through 31.
func Day(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return datePart(ctx, args[0], func(t time.Time) int64 { return int64(t.Day()) })
}

// datePart walks a Date32 or Timestamp array once and builds an int64 array
// of f applied to each non-null value. A zoned timestamp is read in its zone.
func datePart(ctx context.Context, in compute.Datum, f func(time.Time) int64) (compute.Datum, error) {
	ad, ok := in.(*compute.ArrayDatum)
	if !ok {
		return nil, fmt.Errorf("expected array, got %s", in)
	}
	b := array.NewInt64Builder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(ad.Value.Len())

	switch ad.Value.DataType().ID() {
	case arrow.DATE32:
		src := array.NewDate32Data(ad.Value)
		defer src.Release()
		for i := range src.Len() {
			if src.IsNull(i) {
				b.AppendNull()
				continue
			}
			b.UnsafeAppend(f(src.Value(i).ToTime()))
		}
	case arrow.TIMESTAMP:
		src := array.NewTimestampData(ad.Value)
		defer src.Release()
		typ := src.DataType().(*arrow.TimestampType)
		loc, err := typ.GetZone()
		if err != nil {
			return nil, err
		}
		for i := range src.Len() {
			if src.IsNull(i) {
				b.AppendNull()
				continue
			}
			b.UnsafeAppend(f(src.Value(i).ToTime(typ.Unit).In(loc)))
		}
	default:
		return nil, fmt.Errorf("expected date or timestamp array, got %s", ad.Value.DataType())
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}
