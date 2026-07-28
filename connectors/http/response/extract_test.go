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
