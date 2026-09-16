package object

import (
	"encoding/json"
	"fmt"

	"github.com/galaxy-io/filament"
)

// ManifestContentType is the media type for run success markers.
const ManifestContentType = "application/json"

// ResourceResult is immutable input to the run success manifest.
type ResourceResult struct {
	Resource string
	Key      string
	Rows     int64
	Bytes    int64
	CRC32C   uint32
}

type successManifest struct {
	Version   int                `json:"version"`
	Run       string             `json:"run"`
	Resources []manifestResource `json:"resources"`
}

type manifestResource struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	URI    string `json:"uri"`
	Rows   int64  `json:"rows"`
	Bytes  int64  `json:"bytes"`
	CRC32C string `json:"crc32c"`
}

// EncodeManifest builds the common success-marker body for an object store.
func EncodeManifest(run filament.RunID, scheme, bucket string, results []ResourceResult) ([]byte, error) {
	manifest := successManifest{Version: 1, Run: string(run), Resources: make([]manifestResource, len(results))}
	for i, result := range results {
		manifest.Resources[i] = manifestResource{
			Name: result.Resource, Key: result.Key, URI: fmt.Sprintf("%s://%s/%s", scheme, bucket, result.Key),
			Rows: result.Rows, Bytes: result.Bytes, CRC32C: fmt.Sprintf("%08x", result.CRC32C),
		}
	}
	return json.Marshal(manifest)
}
