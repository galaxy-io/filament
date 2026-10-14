package kernel

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
)

// patterns caches compiled expressions by source, so a plan that runs the
// same pattern over every batch compiles it once per process.
var patterns sync.Map

func compilePattern(d compute.Datum) (*regexp.Regexp, error) {
	src, err := stringScalar(d)
	if err != nil {
		return nil, err
	}
	key := string(src)
	if re, ok := patterns.Load(key); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(key)
	if err != nil {
		return nil, fmt.Errorf("regex: %w", err)
	}
	patterns.Store(key, re)
	return re, nil
}

// RegexMatch reports, per row, whether a utf8 column matches the pattern.
func RegexMatch(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	re, err := compilePattern(args[1])
	if err != nil {
		return nil, err
	}
	src, err := stringArray(args[0])
	if err != nil {
		return nil, err
	}
	defer src.Release()
	b := array.NewBooleanBuilder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(src.Len())
	for i := range src.Len() {
		if src.IsNull(i) {
			b.AppendNull()
			continue
		}
		b.UnsafeAppend(re.MatchString(src.Value(i)))
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// RegexExtract returns the first match of the pattern in each value, or the
// given capture group of it. A row with no match is null.
func RegexExtract(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	re, err := compilePattern(args[1])
	if err != nil {
		return nil, err
	}
	group := int64(0)
	if len(args) > 2 {
		if group, err = int64Scalar(args[2]); err != nil {
			return nil, err
		}
	}
	if group < 0 || int(group) > re.NumSubexp() {
		return nil, fmt.Errorf("regex_extract: pattern has %d groups, asked for %d", re.NumSubexp(), group)
	}
	src, err := stringArray(args[0])
	if err != nil {
		return nil, err
	}
	defer src.Release()
	b := array.NewStringBuilder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(src.Len())
	for i := range src.Len() {
		if src.IsNull(i) {
			b.AppendNull()
			continue
		}
		v := src.Value(i)
		loc := re.FindStringSubmatchIndex(v)
		if len(loc) <= int(2*group+1) || loc[2*group] < 0 {
			b.AppendNull()
			continue
		}
		b.Append(v[loc[2*group]:loc[2*group+1]])
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// RegexReplace rewrites every match of the pattern in each value, expanding
// $1-style group references in the replacement.
func RegexReplace(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	re, err := compilePattern(args[1])
	if err != nil {
		return nil, err
	}
	with, err := stringScalar(args[2])
	if err != nil {
		return nil, err
	}
	return mapBytes(ctx, args[0], func(_, s []byte) []byte { return re.ReplaceAll(s, with) })
}
