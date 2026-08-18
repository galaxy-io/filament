package compile

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// routeGroup is the set of edges that share a source node and sink node, and
// so collapse into a single run. Each resource carries its own read mode while
// the write mode is shared by the route; the "" read entry is the route default
// set by an all-resources edge.
type routeGroup struct {
	key           string
	source        *ingestionv1.PipelineNode
	sink          *ingestionv1.PipelineNode
	from          string
	to            string
	readModes     map[string]filament.ReadMode
	writeMode     filament.WriteMode
	all           bool
	resources     map[string]bool
	selectors     map[string]bool
	cursorConfigs map[string]filament.ResourceCursorConfig
}

// groupEdges collapses edges into per-route groups, preserving first-seen order.
// An edge with no resource marks its group as "all resources". Two edges naming
// the same resource (or two all-resources edges) with different read modes
// conflict. All edges in a route must carry the same write mode.
func groupEdges(edges []*ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode) ([]*routeGroup, error) {
	byKey := map[string]*routeGroup{}
	var ordered []*routeGroup
	for _, edge := range edges {
		source := nodes[edge.GetFromNode()]
		sink := nodes[edge.GetToNode()]
		if source == nil || sink == nil {
			return nil, fmt.Errorf("%w: edge references missing node", ErrInvalid)
		}
		readMode, err := readModeFromProto(edge.GetReadMode())
		if err != nil {
			return nil, err
		}
		writeMode, err := writeModeFromProto(edge.GetWriteMode())
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
				readModes:     map[string]filament.ReadMode{},
				writeMode:     writeMode,
				resources:     map[string]bool{},
				selectors:     map[string]bool{},
				cursorConfigs: map[string]filament.ResourceCursorConfig{},
			}
			byKey[key] = group
			ordered = append(ordered, group)
		} else if group.writeMode != writeMode {
			return nil, fmt.Errorf("%w: conflicting write modes for route %s -> %s", ErrInvalid, edge.GetFromNode(), edge.GetToNode())
		}
		for _, cursor := range edge.GetCursors() {
			config := filament.ResourceCursorConfig{Field: cursor.GetField(), LookbackSeconds: cursor.GetLookbackSeconds()}
			if previous, exists := group.cursorConfigs[cursor.GetResource()]; exists && previous != config {
				return nil, fmt.Errorf("%w: conflicting cursor configuration for resource %q", ErrInvalid, cursor.GetResource())
			}
			group.cursorConfigs[cursor.GetResource()] = config
		}
		resource := edge.GetResource()
		if previous, exists := group.readModes[resource]; exists && previous != readMode {
			if resource == "" {
				return nil, fmt.Errorf("%w: conflicting read modes for route %s -> %s", ErrInvalid, edge.GetFromNode(), edge.GetToNode())
			}
			return nil, fmt.Errorf("%w: conflicting read modes for resource %q", ErrInvalid, resource)
		}
		group.readModes[resource] = readMode
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

func readModeFromProto(mode ingestionv1.ReadMode) (filament.ReadMode, error) {
	switch mode {
	case ingestionv1.ReadMode_READ_MODE_UNSPECIFIED, ingestionv1.ReadMode_READ_MODE_FULL:
		return filament.ModeFull, nil
	case ingestionv1.ReadMode_READ_MODE_INCREMENTAL:
		return filament.ModeIncremental, nil
	default:
		return 0, fmt.Errorf("%w: unknown read mode %d", ErrInvalid, mode)
	}
}

func writeModeFromProto(mode ingestionv1.WriteMode) (filament.WriteMode, error) {
	switch mode {
	case ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED, ingestionv1.WriteMode_WRITE_MODE_REPLACE:
		return filament.WriteReplace, nil
	case ingestionv1.WriteMode_WRITE_MODE_APPEND:
		return filament.WriteAppend, nil
	case ingestionv1.WriteMode_WRITE_MODE_UPSERT:
		return filament.WriteUpsert, nil
	default:
		return "", fmt.Errorf("%w: unknown write mode %d", ErrInvalid, mode)
	}
}
