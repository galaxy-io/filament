package compile

import (
	"cmp"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/transform"
)

// routeGroup is the set of edges that share a source node and sink node, and
// so collapse into a single run.
type routeGroup struct {
	key    string
	source *ingestionv1.PipelineNode
	sink   *ingestionv1.PipelineNode
	from   string
	to     string
	// tables holds what the route declares for each resource. The "" entry is
	// the route default set by an all-resources edge.
	tables map[string]*routeTable
}

// routeTable is what a route declares for one resource. An all-resources edge
// can declare settings for a resource no edge names.
type routeTable struct {
	named       bool // an edge names this resource
	readMode    filament.ReadMode
	writeMode   filament.WriteMode
	selector    string
	destination string
	cursor      declared[filament.ResourceCursorConfig]
	columns     declared[[]string] // selected source columns
	partitions  declared[[]string] // partitions to backfill
	sinkConfig  declared[map[string]any]
	transform   declared[any] // steps
}

// declared is a setting an edge may leave out.
type declared[T any] struct {
	value T
	set   bool
}

func declare[T any](value T) declared[T] {
	return declared[T]{value: value, set: true}
}

// table returns the route's entry for resource, adding it when new.
func (g *routeGroup) table(resource string) *routeTable {
	table := g.tables[resource]
	if table == nil {
		table = &routeTable{}
		g.tables[resource] = table
	}
	return table
}

// all reports whether an all-resources edge is on the route.
func (g *routeGroup) all() bool {
	table := g.tables[""]
	return table != nil && table.named
}

// narrow returns the route reading only the named resources, each with
// everything the route declares for it. An all-resources edge reads
// everything, so it names none of them.
func (g *routeGroup) narrow(resources []string) *routeGroup {
	narrowed := *g
	narrowed.tables = map[string]*routeTable{}
	for _, resource := range resources {
		if table := g.tables[resource]; table != nil && table.named {
			narrowed.tables[resource] = table
		}
	}
	return &narrowed
}

// settings returns one setting for every resource that declares it, in the
// shape a run request carries.
func settings[T any](g *routeGroup, setting func(*routeTable) declared[T]) map[string]T {
	out := map[string]T{}
	for resource, table := range g.tables {
		if value := setting(table); value.set {
			out[resource] = value.value
		}
	}
	return out
}

func (g *routeGroup) cursorConfigs() map[string]filament.ResourceCursorConfig {
	return settings(g, func(t *routeTable) declared[filament.ResourceCursorConfig] { return t.cursor })
}

func (g *routeGroup) columns() map[string][]string {
	return settings(g, func(t *routeTable) declared[[]string] { return t.columns })
}

func (g *routeGroup) partitions() map[string][]string {
	return settings(g, func(t *routeTable) declared[[]string] { return t.partitions })
}

func (g *routeGroup) sinkConfigs() map[string]map[string]any {
	return settings(g, func(t *routeTable) declared[map[string]any] { return t.sinkConfig })
}

func (g *routeGroup) transforms() map[string]any {
	return settings(g, func(t *routeTable) declared[any] { return t.transform })
}

func (g *routeGroup) destinations() map[string]string {
	out := map[string]string{}
	for resource, table := range g.tables {
		if table.destination != "" {
			out[resource] = table.destination
		}
	}
	return out
}

// groupEdges collapses edges into per-route groups, preserving first-seen order.
// Two edges naming the same resource (or two all-resources edges) with
// different read or write modes conflict.
func groupEdges(edges []*ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode) ([]*routeGroup, error) {
	if err := ValidateDestinations(edges); err != nil {
		return nil, err
	}
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
		key := routeKey(edge.GetFromNode(), edge.GetToNode())
		group := byKey[key]
		if group == nil {
			group = &routeGroup{key: key, source: source, sink: sink, from: edge.GetFromNode(), to: edge.GetToNode(), tables: map[string]*routeTable{}}
			byKey[key] = group
			ordered = append(ordered, group)
		}
		if err := mergeSettings(group, edge); err != nil {
			return nil, err
		}
		resource := edge.GetResource()
		table := group.table(resource)
		if label := edge.GetDestinationResource(); label != "" {
			table.destination = label
		}
		if table.named && table.readMode != readMode {
			if resource == "" {
				return nil, fmt.Errorf("%w: conflicting read modes for route %s -> %s", ErrInvalid, edge.GetFromNode(), edge.GetToNode())
			}
			return nil, fmt.Errorf("%w: conflicting read modes for resource %q", ErrInvalid, resource)
		}
		if table.named && table.writeMode != writeMode {
			if resource == "" {
				return nil, fmt.Errorf("%w: conflicting write modes for route %s -> %s", ErrInvalid, edge.GetFromNode(), edge.GetToNode())
			}
			return nil, fmt.Errorf("%w: conflicting write modes for resource %q", ErrInvalid, resource)
		}
		table.named, table.readMode, table.writeMode = true, readMode, writeMode
		if resource != "" {
			table.selector = cmp.Or(edge.GetSelector(), resource)
		}
	}
	return ordered, nil
}

// routeWriteMode returns the one write mode every resource on the route uses.
// CDC and continuous routes plan a single write for the whole stream.
func (g *routeGroup) routeWriteMode() (filament.WriteMode, error) {
	var mode filament.WriteMode
	for _, table := range g.tables {
		if !table.named {
			continue
		}
		if mode != "" && table.writeMode != mode {
			return "", fmt.Errorf("%w: all resources on route %s -> %s must use the same write mode", ErrInvalid, g.from, g.to)
		}
		mode = table.writeMode
	}
	return mode, nil
}

// mergeSettings folds what an edge declares per resource into its route. Two
// edges may not give one resource different settings.
func mergeSettings(group *routeGroup, edge *ingestionv1.PipelineEdge) error {
	for _, cursor := range edge.GetCursors() {
		config := filament.ResourceCursorConfig{Field: cursor.GetField(), LookbackSeconds: cursor.GetLookbackSeconds()}
		table := group.table(cursor.GetResource())
		if table.cursor.set && table.cursor.value != config {
			return fmt.Errorf("%w: conflicting cursor configuration for resource %q", ErrInvalid, cursor.GetResource())
		}
		table.cursor = declare(config)
	}
	for _, selection := range edge.GetColumns() {
		columns := slices.Sorted(slices.Values(selection.GetColumns()))
		table := group.table(selection.GetResource())
		if table.columns.set && !slices.Equal(table.columns.value, columns) {
			return fmt.Errorf("%w: conflicting column selection for resource %q", ErrInvalid, selection.GetResource())
		}
		table.columns = declare(columns)
	}
	for _, selection := range edge.GetPartitions() {
		// An empty selection backfills no partition, so it stays distinct from none.
		partitions := append([]string{}, slices.Sorted(slices.Values(selection.GetPartitions()))...)
		table := group.table(selection.GetResource())
		if table.partitions.set && !slices.Equal(table.partitions.value, partitions) {
			return fmt.Errorf("%w: conflicting partition selection for resource %q", ErrInvalid, selection.GetResource())
		}
		table.partitions = declare(partitions)
	}
	for _, sinkConfig := range edge.GetSinkConfigs() {
		config := sinkConfig.GetConfig().AsMap()
		table := group.table(sinkConfig.GetResource())
		if table.sinkConfig.set && !reflect.DeepEqual(table.sinkConfig.value, config) {
			return fmt.Errorf("%w: conflicting sink config for resource %q", ErrInvalid, sinkConfig.GetResource())
		}
		table.sinkConfig = declare(config)
	}
	return mergeTransform(group, edge)
}

// mergeTransform folds an edge's transform into its route. The definition is
// checked against the grammar here so a bad one fails compilation, not the
// run. A resource-specific edge may only define its own resource, and two
// edges may not define the same one.
func mergeTransform(group *routeGroup, edge *ingestionv1.PipelineEdge) error {
	if !HasTransform(edge) {
		return nil
	}
	resources, err := TransformResources(edge)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	for resource, steps := range resources {
		table := group.table(resource)
		if table.transform.set {
			return fmt.Errorf("%w: resource %q is transformed by more than one edge", ErrInvalid, resource)
		}
		table.transform = declare(steps)
	}
	return nil
}

// HasTransform reports whether the edge carries a transform definition. An
// empty object is the same as none: rows pass through unchanged.
func HasTransform(edge *ingestionv1.PipelineEdge) bool {
	return len(edge.GetTransform().GetFields()) > 0
}

// TransformResources parses an edge's transform against the grammar and
// returns its per-resource steps. An edge naming a resource may only define
// that resource.
func TransformResources(edge *ingestionv1.PipelineEdge) (map[string]any, error) {
	raw, err := edge.GetTransform().MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("transform: %w", err)
	}
	if _, err := transform.Parse(raw); err != nil {
		return nil, err
	}
	resources, _ := edge.GetTransform().AsMap()["resources"].(map[string]any)
	if own := edge.GetResource(); own != "" {
		for resource := range resources {
			if resource != own {
				return nil, fmt.Errorf("transform: edge for %q defines resource %q", own, resource)
			}
		}
	}
	return resources, nil
}

// transformDefinition renders a route's merged transforms as the JSON
// definition a run request carries, or "" when the route has none.
func transformDefinition(group *routeGroup) (string, error) {
	transforms := group.transforms()
	if len(transforms) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(map[string]any{"version": transform.GrammarVersion(), "resources": transforms})
	if err != nil {
		return "", fmt.Errorf("transform: %w", err)
	}
	return string(raw), nil
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
	case ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED:
		return filament.WriteUpsert, nil
	case ingestionv1.WriteMode_WRITE_MODE_REPLACE:
		return filament.WriteReplace, nil
	case ingestionv1.WriteMode_WRITE_MODE_APPEND:
		return filament.WriteAppend, nil
	case ingestionv1.WriteMode_WRITE_MODE_UPSERT:
		return filament.WriteUpsert, nil
	case ingestionv1.WriteMode_WRITE_MODE_MERGE:
		return filament.WriteMerge, nil
	default:
		return "", fmt.Errorf("%w: unknown write mode %d", ErrInvalid, mode)
	}
}
