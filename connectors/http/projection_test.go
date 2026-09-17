package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/response"
)

func TestProjectionPreservesJSONNumbers(t *testing.T) {
	extractor := response.New(manifest.ResponseSpec{Root: "array"})
	records, err := extractor.Records([]byte(`[{"id":18446744073709551615,"count":9223372036854775807,"ratio":0.25,"nested":{"id":9007199254740993}}]`))
	if err != nil {
		t.Fatal(err)
	}
	res := manifest.Resource{Fields: manifest.FieldList{
		{Name: "id", Path: "id", Type: "string"},
		{Name: "count", Path: "count", Type: "int64"},
		{Name: "ratio", Path: "ratio", Type: "float64"},
		{Name: "raw", Path: "$", Type: "json", Mode: "remainder"},
	}}
	row, _, err := projectRecord(res, records[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	if row["id"] != "18446744073709551615" || row["count"] != int64(9223372036854775807) || row["ratio"] != 0.25 {
		t.Fatalf("numeric projection: %#v", row)
	}
	raw, err := json.Marshal(row["raw"])
	if err != nil || string(raw) != `{"nested":{"id":9007199254740993}}` {
		t.Fatalf("raw=%s error=%v", raw, err)
	}
	for _, value := range []json.Number{"9223372036854775808", "1.5"} {
		if _, err := scalarInt(value); err == nil {
			t.Fatalf("invalid integer %s was accepted", value)
		}
	}
	for value, want := range map[json.Number]int64{"1.0": 1, "1e3": 1000, "9007199254740993.0": 9007199254740993} {
		got, err := scalarInt(value)
		if err != nil || got != want {
			t.Fatalf("integer %s: %d, %v", value, got, err)
		}
		got, err = paths.Int(map[string]any{"count": value}, "count")
		if err != nil || got != want {
			t.Fatalf("integer path %s: %d, %v", value, got, err)
		}
	}
}
