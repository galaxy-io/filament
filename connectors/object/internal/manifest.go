package object

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/galaxy-io/filament"
)

// ManifestContentType is the media type for run success markers.
const ManifestContentType = "application/json"

// manifestVersion 2 lists every object of a resource under files. Version 1
// carried one key per resource and was replaced when resources began rolling
// into several objects.
const manifestVersion = 2

// ResourceResult is immutable input to the run success manifest. It describes
// one object; a resource that rolled into several objects has one result per
// object, and the manifest groups them by resource.
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
	Name  string         `json:"name"`
	Rows  int64          `json:"rows"`
	Bytes int64          `json:"bytes"`
	Files []manifestFile `json:"files"`
}

type manifestFile struct {
	Key    string `json:"key"`
	URI    string `json:"uri"`
	Rows   int64  `json:"rows"`
	Bytes  int64  `json:"bytes"`
	CRC32C string `json:"crc32c"`
}

// EncodeManifest builds the common success-marker body for an object store.
// Resources are ordered by name and each resource's files in write order.
func EncodeManifest(run filament.RunID, scheme, bucket string, results []ResourceResult) ([]byte, error) {
	sorted := make([]ResourceResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Resource != sorted[j].Resource {
			return sorted[i].Resource < sorted[j].Resource
		}
		return fileOrderLess(sorted[i].Key, sorted[j].Key)
	})
	manifest := successManifest{Version: manifestVersion, Run: string(run), Resources: []manifestResource{}}
	for _, result := range sorted {
		if n := len(manifest.Resources); n == 0 || manifest.Resources[n-1].Name != result.Resource {
			manifest.Resources = append(manifest.Resources, manifestResource{Name: result.Resource, Files: []manifestFile{}})
		}
		resource := &manifest.Resources[len(manifest.Resources)-1]
		resource.Rows += result.Rows
		resource.Bytes += result.Bytes
		resource.Files = append(resource.Files, manifestFile{
			Key: result.Key, URI: fmt.Sprintf("%s://%s/%s", scheme, bucket, result.Key),
			Rows: result.Rows, Bytes: result.Bytes, CRC32C: fmt.Sprintf("%08x", result.CRC32C),
		})
	}
	return json.Marshal(manifest)
}

// fileOrderLess orders one resource's keys by file number. Within a manifest
// those keys differ only in a number padded to at least four digits, so a
// shorter key has the smaller number and equal lengths compare lexically.
func fileOrderLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}
