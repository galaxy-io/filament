package transform

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

var upper = FunctionRef{ID: "fn_upper", Version: 2}

func usersSchema() rowmodel.Schema {
	return rowmodel.Schema{
		Resource: "users",
		Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt64},
			{Name: "email", Logical: rowmodel.LogicalString, Nullable: true},
			{Name: "name", Logical: rowmodel.LogicalString},
		},
	}
}

func keyedUsersSchema() rowmodel.Schema {
	s := usersSchema()
	s.PrimaryKey = []string{"id"}
	return s
}

func arrowFields(fields ...arrow.Field) *arrow.Schema { return arrow.NewSchema(fields, nil) }

func utf8(name string, nullable bool) arrow.Field {
	return arrow.Field{Name: name, Type: arrow.BinaryTypes.String, Nullable: nullable}
}

func alchemyDef(ref FunctionRef) *Definition {
	return &Definition{Version: 1, Resources: map[string]Resource{
		"users": {Steps: []Step{{Alchemy: &AlchemyStep{Function: ref.ID, Version: ref.Version}}}},
	}}
}

// upperContract reads name and writes it back.
func upperContract() ContractMap {
	return ContractMap{upper: {Input: arrowFields(utf8("name", false)), Output: arrowFields(utf8("name", false))}}
}

func TestCompileAlchemyContract(t *testing.T) {
	tests := []struct {
		name      string
		contracts Contracts
		keyed     bool
		wantErr   string
	}{
		{name: "matching contract", contracts: upperContract()},
		{name: "contracts not configured", wantErr: "alchemy functions are not configured"},
		{name: "unknown version", contracts: ContractMap{}, wantErr: "function fn_upper@v2 is not available"},
		{
			name:      "input column missing",
			contracts: ContractMap{upper: {Input: arrowFields(utf8("nickname", true)), Output: arrowFields(utf8("name", false))}},
			wantErr:   `function input "nickname" is not a column of the resource`,
		},
		{
			name: "input type mismatch",
			contracts: ContractMap{upper: {
				Input:  arrowFields(arrow.Field{Name: "name", Type: arrow.PrimitiveTypes.Int32}),
				Output: arrowFields(utf8("name", false)),
			}},
			wantErr: `column "name" is utf8, but the function input is int32`,
		},
		{
			name:      "nullable column into non-null input",
			contracts: ContractMap{upper: {Input: arrowFields(utf8("email", false)), Output: arrowFields(utf8("name", false))}},
			wantErr:   `column "email" is nullable, but the function's input does not accept nulls`,
		},
		{
			name: "output retypes primary key",
			contracts: ContractMap{upper: {
				Input:  arrowFields(utf8("name", false)),
				Output: arrowFields(arrow.Field{Name: "id", Type: arrow.PrimitiveTypes.Int32}),
			}},
			keyed:   true,
			wantErr: `function output "id" is a primary key column and must keep its type int64`,
		},
		{
			name:      "output drops primary key",
			contracts: upperContract(),
			keyed:     true,
			wantErr:   `function output must include primary key column "id"`,
		},
		{
			name: "reserved output",
			contracts: ContractMap{upper: {
				Input:  arrowFields(utf8("name", false)),
				Output: arrowFields(utf8("_filament_run_id", true)),
			}},
			wantErr: `function output "_filament_run_id" is a reserved column`,
		},
		{
			name: "output type with no filament equivalent",
			contracts: ContractMap{upper: {
				Input:  arrowFields(utf8("name", false)),
				Output: arrowFields(arrow.Field{Name: "tags", Type: arrow.ListOf(arrow.BinaryTypes.String), Nullable: true}),
			}},
			wantErr: `function output: field "tags": Arrow type list<item: utf8, nullable> has no filament type`,
		},
		{
			name:      "duplicate output",
			contracts: ContractMap{upper: {Input: arrowFields(utf8("name", false)), Output: arrowFields(utf8("name", false), utf8("name", false))}},
			wantErr:   `function output names "name" more than once`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := usersSchema()
			if tt.keyed {
				schema = keyedUsersSchema()
			}
			plan, err := Compile(alchemyDef(upper), schema, WithContracts(tt.contracts))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Compile() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := plan.Schema(); !slices.Equal(got.Fields, usersSchema().Fields[2:]) {
				t.Fatalf("layout = %+v, want only the output's name", got.Fields)
			}
		})
	}
}

func TestCompileAlchemyOutputReplacesLayout(t *testing.T) {
	json := arrow.Field{
		Name: "profile", Type: arrow.BinaryTypes.String, Nullable: true,
		Metadata: arrow.NewMetadata([]string{arrowbatch.MetaLogical}, []string{"json"}),
	}
	contracts := ContractMap{upper: {
		Input: arrowFields(utf8("name", false)),
		Output: arrowFields(
			arrow.Field{Name: "id", Type: arrow.PrimitiveTypes.Int64},
			utf8("name", true),
			arrow.Field{Name: "email", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
			arrow.Field{Name: "name_len", Type: arrow.PrimitiveTypes.Int32},
			json,
		),
	}}
	plan, err := Compile(alchemyDef(upper), keyedUsersSchema(), WithContracts(contracts))
	if err != nil {
		t.Fatal(err)
	}
	want := []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
		{Name: "email", Logical: rowmodel.LogicalInt64, Nullable: true},
		{Name: "name_len", Logical: rowmodel.LogicalInt32},
		{Name: "profile", Logical: rowmodel.LogicalJSON, Nullable: true},
	}
	got := plan.Schema()
	if !slices.Equal(got.Fields, want) {
		t.Fatalf("fields =\n %+v\nwant\n %+v", got.Fields, want)
	}
	if !slices.Equal(got.PrimaryKey, []string{"id"}) {
		t.Fatalf("primary key = %v", got.PrimaryKey)
	}
}

// fakeInvoker records the batches it is handed and answers with respond.
type fakeInvoker struct {
	calls   []Call
	schemas []*arrow.Schema
	respond func(in arrow.RecordBatch) (arrow.RecordBatch, error)
}

func (f *fakeInvoker) Invoke(_ context.Context, call Call, in arrow.RecordBatch) (arrow.RecordBatch, error) {
	f.calls = append(f.calls, call)
	f.schemas = append(f.schemas, in.Schema())
	return f.respond(in)
}

// upperNames uppercases the one utf8 column of in.
func upperNames(in arrow.RecordBatch) (arrow.RecordBatch, error) {
	src := in.Column(0).(*array.String)
	b := array.NewStringBuilder(memory.DefaultAllocator)
	defer b.Release()
	for i := range src.Len() {
		b.Append(strings.ToUpper(src.Value(i)))
	}
	col := b.NewArray()
	defer col.Release()
	return array.NewRecordBatch(arrowFields(utf8("name", false)), []arrow.Array{col}, int64(col.Len())), nil
}

func userRows(t *testing.T, names ...string) arrow.RecordBatch {
	t.Helper()
	b := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(usersSchema()))
	defer b.Release()
	for i, name := range names {
		b.Field(0).(*array.Int64Builder).Append(int64(i + 1))
		b.Field(1).(*array.StringBuilder).Append(name + "@example.com")
		b.Field(2).(*array.StringBuilder).Append(name)
	}
	rec := b.NewRecordBatch()
	t.Cleanup(rec.Release)
	return rec
}

func compileUpper(t *testing.T, inv Invoker) *Plan {
	t.Helper()
	plan, err := Compile(alchemyDef(upper), usersSchema(), WithContracts(upperContract()), WithInvoker(inv))
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestApplyAlchemyRejectsResultsOutsideTheContract(t *testing.T) {
	tests := []struct {
		name    string
		respond func(in arrow.RecordBatch) (arrow.RecordBatch, error)
		wantErr string
	}{
		{
			name:    "invoker error",
			respond: func(arrow.RecordBatch) (arrow.RecordBatch, error) { return nil, errors.New("boom") },
			wantErr: "fn_upper@v2: boom",
		},
		{
			name: "dropped rows",
			respond: func(in arrow.RecordBatch) (arrow.RecordBatch, error) {
				out, _ := upperNames(in)
				defer out.Release()
				return out.NewSlice(0, 1), nil
			},
			wantErr: "returned 1 rows, want 2",
		},
		{
			name: "wrong type",
			respond: func(in arrow.RecordBatch) (arrow.RecordBatch, error) {
				b := array.NewInt32Builder(memory.DefaultAllocator)
				defer b.Release()
				b.AppendValues([]int32{1, 2}, nil)
				col := b.NewArray()
				defer col.Release()
				return array.NewRecordBatch(arrowFields(arrow.Field{Name: "name", Type: arrow.PrimitiveTypes.Int32}), []arrow.Array{col}, 2), nil
			},
			wantErr: `returned column 0 as "name" int32, want "name" utf8`,
		},
		{
			name: "nulls in a non-null column",
			respond: func(in arrow.RecordBatch) (arrow.RecordBatch, error) {
				b := array.NewStringBuilder(memory.DefaultAllocator)
				defer b.Release()
				b.AppendValues([]string{"A", ""}, []bool{true, false})
				col := b.NewArray()
				defer col.Release()
				return array.NewRecordBatch(arrowFields(utf8("name", false)), []arrow.Array{col}, 2), nil
			},
			wantErr: `returned nulls in non-nullable column "name"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileUpper(t, &fakeInvoker{respond: tt.respond}).Apply(context.Background(), memory.DefaultAllocator, userRows(t, "ada", "grace"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Apply() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestApplyAlchemyReleasesEverything(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.NewGoAllocator())
	defer mem.AssertSize(t, 0)

	b := array.NewRecordBuilder(mem, arrowbatch.Schema(usersSchema()))
	b.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2}, nil)
	b.Field(1).(*array.StringBuilder).AppendValues([]string{"a@x", "b@x"}, nil)
	b.Field(2).(*array.StringBuilder).AppendValues([]string{"a", "b"}, nil)
	rec := b.NewRecordBatch()
	b.Release()

	inv := &fakeInvoker{respond: func(in arrow.RecordBatch) (arrow.RecordBatch, error) {
		sb := array.NewStringBuilder(mem)
		defer sb.Release()
		sb.AppendValues([]string{"A", "B"}, nil)
		col := sb.NewArray()
		defer col.Release()
		return array.NewRecordBatch(arrowFields(utf8("name", false)), []arrow.Array{col}, 2), nil
	}}
	out, err := compileUpper(t, inv).Apply(context.Background(), mem, rec)
	if err != nil {
		t.Fatal(err)
	}
	rec.Release()
	out.Release()
}

func TestApplyAlchemyOutputReplacesRow(t *testing.T) {
	contracts := ContractMap{upper: {
		Input:  arrowFields(utf8("name", false)),
		Output: arrowFields(utf8("name", false), arrow.Field{Name: "name_len", Type: arrow.PrimitiveTypes.Int32}),
	}}
	inv := &fakeInvoker{respond: func(in arrow.RecordBatch) (arrow.RecordBatch, error) {
		src := in.Column(0).(*array.String)
		names := array.NewStringBuilder(memory.DefaultAllocator)
		lens := array.NewInt32Builder(memory.DefaultAllocator)
		defer names.Release()
		defer lens.Release()
		for i := range src.Len() {
			names.Append(strings.ToUpper(src.Value(i)))
			lens.Append(int32(len(src.Value(i))))
		}
		n, l := names.NewArray(), lens.NewArray()
		defer n.Release()
		defer l.Release()
		return array.NewRecordBatch(contracts[upper].Output, []arrow.Array{n, l}, int64(n.Len())), nil
	}}
	plan, err := Compile(alchemyDef(upper), usersSchema(), WithContracts(contracts), WithInvoker(inv))
	if err != nil {
		t.Fatal(err)
	}
	out, err := plan.Apply(context.Background(), memory.DefaultAllocator, userRows(t, "ada", "grace"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Release()
	if want := arrowFields(utf8("name", false)); !inv.schemas[0].Equal(want) {
		t.Fatalf("sent schema = %s, want only the contract's input %s", inv.schemas[0], want)
	}
	if !out.Schema().Equal(arrowbatch.Schema(plan.Schema())) {
		t.Fatalf("record schema %s does not match the plan's %s", out.Schema(), arrowbatch.Schema(plan.Schema()))
	}
	if out.NumCols() != 2 {
		t.Fatalf("columns = %d, want only the output's 2", out.NumCols())
	}
	if got := out.Column(0).(*array.String).Value(1); got != "GRACE" {
		t.Fatalf("name = %q", got)
	}
	if got := out.Column(1).(*array.Int32).Value(1); got != 5 {
		t.Fatalf("name_len = %d", got)
	}
}

func TestApplyAlchemyKeysPassThroughUnchanged(t *testing.T) {
	id := arrow.Field{Name: "id", Type: arrow.PrimitiveTypes.Int64}
	contract := Contract{Input: arrowFields(id, utf8("name", false)), Output: arrowFields(id, utf8("name", false))}
	run := func(shift int64) (arrow.RecordBatch, error) {
		inv := &fakeInvoker{respond: func(in arrow.RecordBatch) (arrow.RecordBatch, error) {
			ids := array.NewInt64Builder(memory.DefaultAllocator)
			names := array.NewStringBuilder(memory.DefaultAllocator)
			defer ids.Release()
			defer names.Release()
			for i, v := range in.Column(0).(*array.Int64).Int64Values() {
				ids.Append(v + shift)
				names.Append(strings.ToUpper(in.Column(1).(*array.String).Value(i)))
			}
			idCol, nameCol := ids.NewArray(), names.NewArray()
			defer idCol.Release()
			defer nameCol.Release()
			return array.NewRecordBatch(contract.Output, []arrow.Array{idCol, nameCol}, in.NumRows()), nil
		}}
		plan, err := Compile(alchemyDef(upper), keyedUsersSchema(), WithContracts(ContractMap{upper: contract}), WithInvoker(inv))
		if err != nil {
			t.Fatal(err)
		}
		return plan.Apply(context.Background(), memory.DefaultAllocator, userRows(t, "ada", "grace"))
	}

	out, err := run(0)
	if err != nil {
		t.Fatalf("an unchanged key failed: %v", err)
	}
	if got := out.Column(1).(*array.String).Value(0); got != "ADA" {
		t.Fatalf("name = %q", got)
	}
	out.Release()
	if _, err := run(1); err == nil || !strings.Contains(err.Error(), `changed primary key column "id"`) {
		t.Fatalf("a changed key: error = %v", err)
	}
}
