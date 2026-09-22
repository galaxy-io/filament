package compile

import (
	"fmt"
	"strings"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ValidateContinuousDestinations keeps source membership separate from destination
// naming and rejects ambiguous routes before any connector I/O.
func ValidateContinuousDestinations(edges []*ingestionv1.PipelineEdge) error {
	seen := map[string]bool{}
	resources := map[string]bool{}
	for _, edge := range edges {
		label := edge.GetDestinationResource()
		if label != "" && (edge.GetResource() == "" || strings.TrimSpace(label) != label || strings.ContainsRune(label, 0)) {
			return fmt.Errorf("%w: destination label requires a named resource and must not contain surrounding whitespace or NUL", ErrInvalid)
		}
		if label == "" {
			label = edge.GetResource()
		}
		if label == "" {
			if edge.GetResource() != "" {
				return fmt.Errorf("%w: resource requires a destination label", ErrInvalid)
			}
			continue
		}
		target := edge.GetToNode() + "\x00" + label
		source := edge.GetFromNode() + "\x00" + edge.GetToNode() + "\x00" + edge.GetResource()
		if seen[target] || resources[source] {
			return fmt.Errorf("%w: duplicate source resource or destination label %q", ErrInvalid, label)
		}
		seen[target] = true
		resources[source] = true
	}
	return nil
}
