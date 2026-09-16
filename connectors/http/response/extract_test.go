package response

import (
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
