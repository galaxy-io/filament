package source

import (
	"context"

	"github.com/galaxy-io/filament/rowmodel"
)

// Schema returns transport and event metadata; payload fields are inferred on the first read.
func (s *Source) Schema(_ context.Context, resource string) (rowmodel.Schema, error) {
	return rowmodel.WithEventMetadataFields(messageBaseSchema(resource))
}
