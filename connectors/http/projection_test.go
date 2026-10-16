package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"

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

func TestProjectionValuesTuplesAndExactDecimals(t *testing.T) {
	const data = `version: 1
name: typed
display_name: Typed
description: Typed projection fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  account: {type: string, default: '42'}
connection: {base_url: https://example.com}
resources:
  - name: rows
    path: /rows
    primary_key: [account, id]
    query: {from: '2026-01-01', empty: ''}
    fields:
      account: {type: string, value: '{{ config.account }}'}
      id: {type: string, tuple: ['literal:Snapshot', name, 'query:from,empty']}
      amount: {path: amount, type: decimal, precision: 38, scale: 18}
      rate: {path: rate, type: decimal, precision: 38, scale: 20}
      nested: json
      raw: {path: $, type: json, mode: remainder}
`
	src := NewManifest([]byte(data))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	res := src.connector.manifest.Resources[0]
	records, err := response.New(manifest.ResponseSpec{Root: "array"}).Records([]byte(`[{"name":"a|b","amount":9007199254740993.123456789012345678,"rate":1.23456789012345678901,"nested":{"number":9007199254740993},"extra":1}]`))
	if err != nil {
		t.Fatal(err)
	}
	row, _, err := projectRecord(res, records[0], nil, src.connector.creds)
	if err != nil {
		t.Fatal(err)
	}
	if row["account"] != "42" || row["id"] != `["Snapshot","a|b","from=2026-01-01"]` {
		t.Fatalf("identity: %v", row)
	}
	payload, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	sink := &collectSink{}
	writer := newRowSink(t.Context(), src, sink)
	if err := writer.Push(record{Resource: "rows", Data: payload}); err != nil {
		t.Fatal(err)
	}
	text := string(sink.records[0].Data)
	for _, number := range []string{"9007199254740993.123456789012345678", "1.23456789012345678901", "9007199254740993"} {
		if !strings.Contains(text, number) {
			t.Fatalf("precision lost: %s", text)
		}
	}
	for _, value := range []string{"0.0000000000000000001", "100000000000000000000", "1e999999", "1/2"} {
		row["amount"] = value
		payload, _ = json.Marshal(row)
		// A failed row leaves its builder unusable, so use a fresh one per case.
		if err := newRowSink(t.Context(), src, &collectSink{}).Push(record{Resource: "rows", Data: payload}); err == nil {
			t.Fatalf("accepted invalid decimal %s", value)
		}
	}
}

type responseMatchSink struct{ count int }

func (s *responseMatchSink) Push(record) error                { s.count++; return nil }
func (s *responseMatchSink) PushBatch(records []record) error { s.count += len(records); return nil }

func TestResponseMatchChecksConfigAndParentBeforeEmission(t *testing.T) {
	c := &Connector{creds: map[string]string{"account": "expected"}}
	res := manifest.Resource{Name: "items", Response: manifest.ResponseSpec{Match: map[string]string{"account": "{{ config.account }}", "code": "{{ parent.code }}"}}}
	for _, row := range []map[string]any{
		{"account": "expected", "code": "EUR"},
		{"account": "wrong", "code": "EUR"},
		{"account": "expected", "code": "USD"},
		{"account": "expected"},
	} {
		sink := &responseMatchSink{}
		_, _, err := c.sendRecords(res, []map[string]any{row}, sink, Capture{"code": "EUR"}, "", response.New(res.Response), nil)
		valid := row["account"] == "expected" && row["code"] == "EUR"
		if (err == nil) != valid || (sink.count == 1) != valid {
			t.Fatalf("row=%v count=%d err=%v", row, sink.count, err)
		}
	}
}
