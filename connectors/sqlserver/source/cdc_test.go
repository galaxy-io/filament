package sqlserver

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament/rowmodel"
)

func mustLSN(t *testing.T, s string) lsn {
	t.Helper()
	l, err := parseLSN(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestParseLSN(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0000002b00000f380001", "0000002b00000f380001"},
		{"0x0000002B00000F380001", "0000002b00000f380001"},
		{"0000002B:00000F38:0001", "0000002b00000f380001"},
		{"2B:F38:1", "0000002b00000f380001"},
	} {
		if got := mustLSN(t, tc.in).String(); got != tc.want {
			t.Errorf("parseLSN(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"", "0x00", "zz00002b00000f380001", "1:2", "000000002B:F38:1"} {
		if _, err := parseLSN(bad); err == nil {
			t.Errorf("parseLSN(%q) accepted", bad)
		}
	}
}

func TestLSNNextUndoesPrev(t *testing.T) {
	for _, s := range []string{"00000010000000000001", "000000100000000000ff", "0000001000000000ff00", "00000000000000000000"} {
		l := mustLSN(t, s)
		if got := l.prev().next(); got != l && !l.isZero() {
			t.Errorf("%s: prev then next = %s", s, got)
		}
		if got := l.next().prev(); got != l {
			t.Errorf("%s: next then prev = %s", s, got)
		}
	}
}

func TestLSNOrderAndPrev(t *testing.T) {
	a, b := mustLSN(t, "0000002b00000f380001"), mustLSN(t, "0000002c000000100003")
	if a.compare(b) >= 0 || b.compare(a) <= 0 || a.compare(a) != 0 {
		t.Fatal("lsn order")
	}
	if a.String() >= b.String() {
		t.Fatal("text form must sort in log order")
	}
	if got := mustLSN(t, "0000002c000000100000").prev().String(); got != "0000002c0000000fffff" {
		t.Fatalf("prev borrow = %s", got)
	}
	if got := mustLSN(t, "0000002b00000f380001").prev(); got.compare(a) >= 0 {
		t.Fatalf("prev not below: %s", got)
	}
	if !(lsn{}).prev().isZero() {
		t.Fatal("prev of zero")
	}
}

func cdcTestTable(fields ...string) *table {
	t := &table{name: "orders", qualified: "[dbo].[orders]"}
	for _, f := range fields {
		t.cols = append(t.cols, column{name: f, typeName: "int"})
		t.schema.Fields = append(t.schema.Fields, rowmodel.Field{Name: f, Logical: rowmodel.LogicalInt32})
	}
	t.schema.PrimaryKey = []string{fields[0]}
	return t
}

func instance(t *testing.T, name, start string, cols ...string) captureInstance {
	inst := captureInstance{name: name, ct: "[cdc].[" + name + "_CT]", start: mustLSN(t, start), cols: map[string]column{}, commandID: true}
	for _, c := range cols {
		inst.cols[c] = column{name: c, typeName: "int"}
	}
	return inst
}

func TestPlanRead(t *testing.T) {
	tbl := cdcTestTable("id", "v")
	old := instance(t, "dbo_orders", "00000010000000000001", "id")
	next := instance(t, "dbo_orders_v2", "00000020000000000001", "id", "v")

	plan, err := planRead(tbl, []captureInstance{old, next}, mustLSN(t, "00000015000000000000"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.inst.name != old.name || plan.stop != next.start || !plan.rotating {
		t.Fatalf("rotation plan = %+v", plan)
	}
	plan, err = planRead(tbl, []captureInstance{old, next}, mustLSN(t, "00000020000000000001"))
	if err != nil || plan.inst.name != next.name || !plan.stop.isZero() {
		t.Fatalf("newest plan = %+v, %v", plan, err)
	}
	if _, err := planRead(tbl, []captureInstance{old, next}, mustLSN(t, "00000001000000000000")); err == nil || !strings.Contains(err.Error(), "no longer in capture instance") {
		t.Fatalf("purged position: %v", err)
	}
	// A checkpoint just before the low watermark still has every change it
	// needs: cleanup deletes only below the watermark.
	if plan, err := planRead(tbl, []captureInstance{next}, next.start.prev()); err != nil || plan.inst.name != next.name {
		t.Fatalf("checkpoint just before the low watermark = %+v, %v", plan, err)
	}
	if _, err := planRead(tbl, []captureInstance{next}, next.start.prev().prev()); err == nil {
		t.Fatal("accepted a checkpoint two positions before the low watermark")
	}
	if _, err := planRead(tbl, []captureInstance{old}, mustLSN(t, "00000015000000000000")); err == nil || !strings.Contains(err.Error(), `does not capture column "v"`) {
		t.Fatalf("uncaptured column: %v", err)
	}
}

func TestChangeQuery(t *testing.T) {
	a := &cdcTable{t: cdcTestTable("id", "v"), floor: mustLSN(t, "00000010000000000001")}
	ia := instance(t, "dbo_a", "00000001000000000001", "id", "v")
	a.plan = readPlan{inst: &ia}
	b := &cdcTable{t: cdcTestTable("k"), floor: mustLSN(t, "00000012000000000001")}
	b.t.cols[0].typeName = "hierarchyid"
	b.t.schema.Fields[0].Logical = rowmodel.LogicalString
	ib := instance(t, "dbo_b", "00000001000000000001")
	ib.cols["k"] = column{name: "k", typeName: "hierarchyid"}
	ib.commandID = false
	b.plan = readPlan{inst: &ib, stop: mustLSN(t, "00000014000000000000")}

	end := mustLSN(t, "00000020000000000000")
	q, args := changeQuery([]*cdcTable{a, b}, end)
	for _, want := range []string{
		"SELECT [__$start_lsn] AS [s], [__$command_id] AS [c], [__$seqval] AS [q], [__$operation] AS [o], 0 AS [t], [id] AS [f0_0], [v] AS [f0_1], CAST(NULL AS hierarchyid).ToString() AS [f1_0] FROM [cdc].[dbo_a_CT] WHERE [__$start_lsn] > @p1 AND [__$start_lsn] <= @p2",
		"UNION ALL\nSELECT [__$start_lsn] AS [s], CAST(0 AS int) AS [c], [__$seqval] AS [q], [__$operation] AS [o], 1 AS [t], CAST(NULL AS int), CAST(NULL AS int), [k].ToString() FROM [cdc].[dbo_b_CT] WHERE [__$start_lsn] > @p3 AND [__$start_lsn] <= @p4",
		") x ORDER BY [s], [c], [q], [o]",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q:\n%s", want, q)
		}
	}
	if len(args) != 4 {
		t.Fatalf("args = %d", len(args))
	}
	if got := lsn(args[3].([]byte)); got != b.plan.stop {
		t.Fatalf("rotating table read past its stop: %s", got)
	}
	if got := lsn(args[1].([]byte)); got != end {
		t.Fatalf("upper bound = %s", got)
	}
}

func TestChangeGroups(t *testing.T) {
	wide := func() *cdcTable {
		names := make([]string, 1500)
		for i := range names {
			names[i] = "c" + strings.Repeat("x", i%3) + string(rune('a'+i%26))
		}
		return &cdcTable{t: cdcTestTable(names...)}
	}
	groups := changeGroups([]*cdcTable{wide(), wide(), wide(), wide()})
	if len(groups) != 2 || len(groups[0]) != 2 || len(groups[1]) != 2 {
		t.Fatalf("groups = %d", len(groups))
	}
}

func TestCaptureIdleSince(t *testing.T) {
	baseline := captureProgress{session: 2, empty: 4}
	for _, tc := range []struct {
		name string
		now  captureProgress
		want bool
	}{
		{"same scan still counting", captureProgress{session: 2, empty: 5}, false},
		{"two more empty scans", captureProgress{session: 2, empty: 6}, true},
		{"fresh empty session", captureProgress{session: 3, empty: 1, fresh: true}, true},
		{"fresh session that captured", captureProgress{session: 3, empty: 0, fresh: true}, false},
		{"no scan yet", captureProgress{}, false},
	} {
		if got := tc.now.idleSince(baseline); got != tc.want {
			t.Errorf("%s: idleSince = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// change is a merge row whose commit LSN ends in at.
func change(at byte) *bufferedChange {
	var commit, seq [10]byte
	commit[9], seq[9] = at, at
	return &bufferedChange{
		values: [][]byte{commit[:], {0, 0, 0, 0}, seq[:], {2, 0, 0, 0}},
		null:   make([]bool, 4),
	}
}

func feed(rows ...byte) func(context.Context, chan<- *bufferedChange) error {
	return func(ctx context.Context, out chan<- *bufferedChange) error {
		for _, r := range rows {
			select {
			case out <- change(r):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
}

func merge(t *testing.T, producers []func(context.Context, chan<- *bufferedChange) error, apply func(int, *bufferedChange) error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- mergeChanges(t.Context(), len(producers), func(ctx context.Context, i int, out chan<- *bufferedChange) error {
			return producers[i](ctx, out)
		}, apply)
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("merge did not return")
		return nil
	}
}

func TestMergeChangesKeepsChangeOrder(t *testing.T) {
	var got []byte
	err := merge(t, []func(context.Context, chan<- *bufferedChange) error{feed(1, 4, 5), feed(2, 3, 6)}, func(_ int, c *bufferedChange) error {
		got = append(got, c.values[0][9])
		return nil
	})
	if err != nil || !bytes.Equal(got, []byte{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("merged %v, %v", got, err)
	}
}

func TestMergeChangesReportsAFailedQueryOverTheLimit(t *testing.T) {
	failed := errors.New("query failed")
	applied := 0
	err := merge(t, []func(context.Context, chan<- *bufferedChange) error{
		func(context.Context, chan<- *bufferedChange) error { return failed },
		feed(1, 2, 3),
	}, func(int, *bufferedChange) error {
		if applied++; applied == 2 {
			return errLimit
		}
		return nil
	})
	if !errors.Is(err, failed) || errors.Is(err, errLimit) {
		t.Fatalf("merge = %v, want the query's failure and not the limit", err)
	}
}

func TestMergeChangesStopsWithoutWaitingForAStalledQuery(t *testing.T) {
	stalled := func(ctx context.Context, out chan<- *bufferedChange) error {
		out <- change(1)
		<-ctx.Done() // never sends another row until the merge cancels it
		return ctx.Err()
	}
	sink := errors.New("sink failed")
	if err := merge(t, []func(context.Context, chan<- *bufferedChange) error{stalled, feed(2)}, func(int, *bufferedChange) error { return sink }); !errors.Is(err, sink) {
		t.Fatalf("merge = %v, want the sink's failure", err)
	}
	if err := merge(t, []func(context.Context, chan<- *bufferedChange) error{stalled, feed(2)}, func(int, *bufferedChange) error { return errLimit }); !errors.Is(err, errLimit) {
		t.Fatalf("merge = %v, want the limit", err)
	}
}
