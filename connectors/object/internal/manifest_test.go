package object

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestEncodeManifestGroupsFilesByResource(t *testing.T) {
	body, err := EncodeManifest("run", "s3", "bucket", []ResourceResult{
		{Resource: "orders", Key: "p/orders/run.0001.ndjson", Rows: 2, Bytes: 20, CRC32C: 0xb},
		{Resource: "accounts", Key: "p/accounts/run.0000.ndjson", Rows: 5, Bytes: 50, CRC32C: 0xa},
		{Resource: "orders", Key: "p/orders/run.0000.ndjson", Rows: 3, Bytes: 30, CRC32C: 0xc},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":2,"run":"run","resources":[` +
		`{"name":"accounts","rows":5,"bytes":50,"files":[` +
		`{"key":"p/accounts/run.0000.ndjson","uri":"s3://bucket/p/accounts/run.0000.ndjson","rows":5,"bytes":50,"crc32c":"0000000a"}]},` +
		`{"name":"orders","rows":5,"bytes":50,"files":[` +
		`{"key":"p/orders/run.0000.ndjson","uri":"s3://bucket/p/orders/run.0000.ndjson","rows":3,"bytes":30,"crc32c":"0000000c"},` +
		`{"key":"p/orders/run.0001.ndjson","uri":"s3://bucket/p/orders/run.0001.ndjson","rows":2,"bytes":20,"crc32c":"0000000b"}]}]}`
	if string(body) != want {
		t.Fatalf("manifest =\n%s\nwant\n%s", body, want)
	}

	empty, err := EncodeManifest("run", "gs", "bucket", nil)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(empty, &decoded); err != nil || decoded["resources"] == nil {
		t.Fatalf("empty manifest = %s, %v; resources must be an empty list, not null", empty, err)
	}
}

// TestEncodeManifestKeepsFileOrderPastPadding covers the file number growing
// past the four-digit padding, where plain string order would put .10000
// before .9999.
func TestEncodeManifestKeepsFileOrderPastPadding(t *testing.T) {
	body, err := EncodeManifest("run", "s3", "bucket", []ResourceResult{
		{Resource: "events", Key: "p/events/run.10000.ndjson"},
		{Resource: "events", Key: "p/events/run.9999.ndjson"},
		{Resource: "events", Key: "p/events/run.0001.ndjson"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Resources []struct {
			Files []struct {
				Key string `json:"key"`
			} `json:"files"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, file := range manifest.Resources[0].Files {
		keys = append(keys, file.Key)
	}
	want := []string{"p/events/run.0001.ndjson", "p/events/run.9999.ndjson", "p/events/run.10000.ndjson"}
	if !slices.Equal(keys, want) {
		t.Fatalf("file order = %v, want %v", keys, want)
	}
}
