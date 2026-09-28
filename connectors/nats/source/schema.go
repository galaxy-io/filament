package source

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

func messageBaseSchema(resource string) rowmodel.Schema {
	return rowmodel.Schema{Resource: resource, DestinationResource: destinationResource(resource), Fields: []rowmodel.Field{{Name: "subject", Logical: rowmodel.LogicalString}}}
}

// Schema returns transport and event metadata; payload fields are inferred on the first read.
func Schema(resource string) (rowmodel.Schema, error) {
	return rowmodel.WithEventMetadataFields(messageBaseSchema(resource))
}

func messageHeaders(header nats.Header) []streamkit.Header {
	if header == nil {
		return nil
	}
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	headers := make([]streamkit.Header, 0)
	for _, key := range keys {
		for _, value := range header[key] {
			headers = append(headers, streamkit.Header{Key: key, Value: []byte(value)})
		}
	}
	return headers
}

// Schema returns transport and event metadata; payload fields are inferred on the first read.
func (s *Source) Schema(_ context.Context, resource string) (rowmodel.Schema, error) {
	return Schema(resource)
}

var resourceNameSeparators = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// destinationResource formats an output name without changing the subscription subject.
func destinationResource(resource string) string {
	return strings.Trim(resourceNameSeparators.ReplaceAllString(resource, "_"), "_")
}

// DestinationResource resolves a subject's output name without fetching a schema.
func (*Source) DestinationResource(resource string) string {
	return destinationResource(resource)
}
