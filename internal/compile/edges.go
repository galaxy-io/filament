package compile

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// routeGroup is the set of edges that share a source node and sink node, and
// so collapse into a single run. Each resource carries its own Standard sync mode;
// the "" entry is the route default set by an all-resources edge.
type routeGroup struct {
	key           string
	source        *ingestionv1.PipelineNode
	sink          *ingestionv1.PipelineNode
	from          string
	to            string
	syncModes     map[string]filament.StandardSyncMode
	all           bool
	resources     map[string]bool
	selectors     map[string]bool
	cursorConfigs map[string]filament.ResourceCursorConfig
}

// groupEdges collapses edges into per-route groups, preserving first-seen order.
// An edge with no resource marks its group as "all resources". Two edges naming
// the same resource (or two all-resources edges) with different sync modes
// conflict.
func groupEdges(edges []*ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode) ([]*routeGroup, error) {
	byKey := map[string]*routeGroup{}
	var ordered []*routeGroup
	for _, edge := range edges {
		source := nodes[edge.GetFromNode()]
		sink := nodes[edge.GetToNode()]
		if source == nil || sink == nil {
			return nil, fmt.Errorf("%w: edge references missing node", ErrInvalid)
		}
		syncMode, err := standardSyncModeFromProto(edge.GetStandardSyncMode())
		if err != nil {
			return nil, err
		}
		key := fmt.Sprintf("route/%s/%s", edge.GetFromNode(), edge.GetToNode())
		group := byKey[key]
		if group == nil {
			group = &routeGroup{
				key:           key,
				source:        source,
				sink:          sink,
				from:          edge.GetFromNode(),
				to:            edge.GetToNode(),
				syncModes:     map[string]filament.StandardSyncMode{},
				resources:     map[string]bool{},
				selectors:     map[string]bool{},
				cursorConfigs: map[string]filament.ResourceCursorConfig{},
			}
			byKey[key] = group
			ordered = append(ordered, group)
		}
		for _, cursor := range edge.GetCursors() {
			config := filament.ResourceCursorConfig{Field: cursor.GetField(), LookbackSeconds: cursor.GetLookbackSeconds()}
			if previous, exists := group.cursorConfigs[cursor.GetResource()]; exists && previous != config {
				return nil, fmt.Errorf("%w: conflicting cursor configuration for resource %q", ErrInvalid, cursor.GetResource())
			}
			group.cursorConfigs[cursor.GetResource()] = config
		}
		resource := edge.GetResource()
		if previous, exists := group.syncModes[resource]; exists && previous != syncMode {
			if resource == "" {
				return nil, fmt.Errorf("%w: conflicting sync modes for route %s -> %s", ErrInvalid, edge.GetFromNode(), edge.GetToNode())
			}
			return nil, fmt.Errorf("%w: conflicting sync modes for resource %q", ErrInvalid, resource)
		}
		group.syncModes[resource] = syncMode
		if resource == "" {
			group.all = true
			continue
		}
		group.resources[resource] = true
		if selector := edge.GetSelector(); selector != "" {
			group.selectors[selector] = true
		} else {
			group.selectors[resource] = true
		}
	}
	return ordered, nil
}

func standardSyncModeFromProto(mode ingestionv1.StandardSyncMode) (filament.StandardSyncMode, error) {
	switch mode {
	case ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_UNSPECIFIED,
		ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE:
		return filament.StandardSyncReplace, nil
	case ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_APPEND:
		return filament.StandardSyncAppend, nil
	case ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL:
		return filament.StandardSyncIncremental, nil
	default:
		return "", fmt.Errorf("%w: unknown Standard sync mode %d", ErrInvalid, mode)
	}
}
