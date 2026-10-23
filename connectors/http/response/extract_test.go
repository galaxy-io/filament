package response

import (
	"encoding/json"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestRecordsSupportsSingleObjectCardinality(t *testing.T) {
	extractor := New(manifest.ResponseSpec{
		RecordsPath: "data",
		Cardinality: "one",
	})
	records, err := extractor.Records([]byte(`{"data":{"id":"widget-1","name":"Widget"}}`))
	if err != nil {
		t.Fatalf("extract one record: %v", err)
	}
	if len(records) != 1 || records[0]["id"] != "widget-1" {
		t.Fatalf("records = %#v, want one widget", records)
	}
}

func TestRecordsTreatsMissingOrNullPathAsEmpty(t *testing.T) {
	extractor := New(manifest.ResponseSpec{RecordsPath: "data"})
	for name, body := range map[string]string{
		"missing": `{"success":true}`,
		"null":    `{"success":true,"data":null}`,
		"empty":   `{"success":true,"data":[]}`,
	} {
		records, err := extractor.Records([]byte(body))
		if err != nil || len(records) != 0 {
			t.Fatalf("%s: records=%#v err=%v", name, records, err)
		}
	}
	if _, err := extractor.Records([]byte(`{"success":true,"data":{"id":1}}`)); err == nil {
		t.Fatal("object at records path was accepted as a page")
	}
	if _, err := extractor.Records([]byte(`{"success":true,"data":"none"}`)); err == nil {
		t.Fatal("string at records path was accepted as a page")
	}
}

func TestStrictEnvelopeAndTaggedUnion(t *testing.T) {
	spec := manifest.ResponseSpec{RecordsPath: "Result.Templates", RequiredObjects: []string{"Result"}, StrictRecords: true, EmptyObject: "Result", Unwrap: &manifest.UnwrapSpec{Variants: []string{"Sale", "Purchase"}, Tag: "kind", Remainder: "wrapper"}}
	extractor := New(spec)
	rows, err := extractor.Records([]byte(`{"Result":{"Templates":[{"Sale":{"Id":"1","amount":9007199254740993.123},"schedule":{"enabled":true}}]}}`))
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0]["kind"] != "Sale" || rows[0]["Id"] != "1" || rows[0]["amount"] != json.Number("9007199254740993.123") || rows[0]["wrapper"] == nil {
		t.Fatalf("union lost data: %v", rows[0])
	}
	for _, body := range []string{`{}`, `{"Result":null}`, `{"Result":{"Templates":null}}`, `{"Result":{"Other":[]}}`, `{"Result":{"Templates":[null]}}`, `{"Result":{"Templates":[{"Sale":{},"Purchase":{}}]}}`, `{"Result":{"Templates":[{"Unknown":{}}]}}`, `{"Result":{"Templates":[{"Sale":{"kind":"overwrite"}}]}}`} {
		if _, err := extractor.Records([]byte(body)); err == nil {
			t.Fatalf("accepted malformed envelope %s", body)
		}
	}
	if rows, err := extractor.Records([]byte(`{"Result":{}}`)); err != nil || len(rows) != 0 {
		t.Fatalf("valid empty page: %v", err)
	}
	singleton := New(manifest.ResponseSpec{RecordsPath: "Settings", Cardinality: "one", StrictRecords: true})
	if _, err := singleton.Records([]byte(`{}`)); err == nil {
		t.Fatal("missing singleton accepted")
	}
	// Existing permissive manifests retain their missing/null-page behavior.
	if rows, err := New(manifest.ResponseSpec{RecordsPath: "data"}).Records([]byte(`{}`)); err != nil || len(rows) != 0 {
		t.Fatal(err)
	}
}

func TestSingletonTaggedUnion(t *testing.T) {
	extractor := New(manifest.ResponseSpec{RecordsPath: "result", Cardinality: "one", StrictRecords: true, Unwrap: &manifest.UnwrapSpec{Variants: []string{"Sale"}, Tag: "kind", Remainder: "wrapper"}})
	rows, err := extractor.Records([]byte(`{"result":{"Sale":{"Id":"1"},"schedule":true}}`))
	if err != nil || len(rows) != 1 || rows[0]["kind"] != "Sale" || rows[0]["wrapper"] == nil {
		t.Fatalf("singleton union: %v %v", rows, err)
	}
	for _, body := range []string{`{"result":{"Sale":null}}`, `{"result":{"Sale":{"wrapper":"collision"}}}`} {
		if _, err := extractor.Records([]byte(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
