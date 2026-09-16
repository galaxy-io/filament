package object

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

// CompressionOptions returns the UI enum options for a file format.
func CompressionOptions(format encoder.FileFormat) []filament.EnumOption {
	compressions := format.SupportedCompressions()
	options := make([]filament.EnumOption, len(compressions))
	for i, compression := range compressions {
		options[i] = filament.EnumOption{Value: string(compression), Label: compressionLabel(compression)}
	}
	return options
}

func compressionLabel(compression encoder.Compression) string {
	switch compression {
	case encoder.CompressionNone:
		return "None"
	case encoder.CompressionGZIP:
		return "Gzip"
	case encoder.CompressionSnappy:
		return "Snappy"
	default:
		return string(compression)
	}
}

// CommitDurableCapabilities describes object writes published at Commit.
func CommitDurableCapabilities(types ...filament.IngestionType) []filament.WritePolicyCapability {
	capabilities := filament.WriteCapabilities(types...)
	for i := range capabilities {
		capabilities[i].Durability = filament.DurabilityAfterCommit
		capabilities[i].Atomicity = filament.AtomicityResource
	}
	return capabilities
}
