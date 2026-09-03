package model

import (
	"fmt"
	"maps"
	"slices"
)

// PipelineGraph is the lossless, target-neutral form of a deployed pipeline.
// Simple pipelines project into it at target boundaries.
type PipelineGraph struct {
	Nodes []PipelineGraphNode
	Edges []PipelineGraphEdge
}

// PipelineGraphNode references one saved connection.
type PipelineGraphNode struct {
	ID         string
	Kind       string
	Connection string
	Config     map[string]any
	SecretRefs map[string]string
}

// PipelineGraphEdge carries every routing setting the deployed API exposes.
// ReadMode is empty on CDC routes, which have no read lever.
type PipelineGraphEdge struct {
	From      string
	To        string
	Resource  string
	Selector  string
	Cursors   []ResourceCursor
	ReadMode  string
	WriteMode string
}

// ResourceCursor is one durable incremental cursor override.
type ResourceCursor struct {
	Resource        string
	Field           string
	LookbackSeconds int64
}

// SyncModeCDC is the simple pipeline's sync mode for a CDC source. The graph
// carries no read mode for it; the source connection's replication does.
const SyncModeCDC = "cdc"

// ProjectSimpleGraph fills the simple pipeline fields from graph when the
// graph is one source-to-sink route with uniform modes and no selectors or
// cursor overrides. The graph is retained either way so view and run stay
// available when the projection is refused.
func ProjectSimpleGraph(pipeline *Pipeline, graph PipelineGraph, sourceReplication string) error {
	pipeline.Graph = cloneGraph(graph)
	var source, sink *PipelineGraphNode
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		switch node.Kind {
		case "source":
			if source != nil {
				return fmt.Errorf("multiple source nodes")
			}
			source = node
		case "sink":
			if sink != nil {
				return fmt.Errorf("multiple sink nodes")
			}
			sink = node
		default:
			return fmt.Errorf("node %q has unknown kind %q", node.ID, node.Kind)
		}
	}
	if len(graph.Nodes) != 2 || source == nil || sink == nil {
		return fmt.Errorf("not a single source-to-sink route")
	}
	if len(graph.Edges) == 0 {
		return fmt.Errorf("no route")
	}
	readMode, writeMode := "", ""
	var resources []string
	seen := map[string]bool{}
	for i, edge := range graph.Edges {
		if edge.From != source.ID || edge.To != sink.ID {
			return fmt.Errorf("more than one source-to-sink route")
		}
		if edge.Selector != "" || len(edge.Cursors) > 0 {
			return fmt.Errorf("selectors or cursor overrides in use")
		}
		if edge.Resource == "" && len(graph.Edges) != 1 {
			return fmt.Errorf("all-resource route mixed with resource routes")
		}
		if seen[edge.Resource] {
			return fmt.Errorf("duplicate route for resource %q", edge.Resource)
		}
		seen[edge.Resource] = true
		if i == 0 {
			readMode, writeMode = edge.ReadMode, edge.WriteMode
		} else if edge.ReadMode != readMode {
			return fmt.Errorf("read mode differs by resource")
		} else if edge.WriteMode != writeMode {
			return fmt.Errorf("write mode differs by resource")
		}
		if edge.Resource != "" {
			resources = append(resources, edge.Resource)
		}
	}
	switch {
	case sourceReplication == SyncModeCDC:
		if readMode != "" {
			return fmt.Errorf("read mode set on a CDC source")
		}
		readMode = SyncModeCDC
	case readMode == "":
		readMode = "full"
	}
	pipeline.Source = PipelineNode{Ref: source.Connection, Config: cloneAnyMap(source.Config), SecretRefs: maps.Clone(source.SecretRefs)}
	pipeline.Sink = PipelineNode{Ref: sink.Connection, Config: cloneAnyMap(sink.Config), SecretRefs: maps.Clone(sink.SecretRefs)}
	pipeline.Resources = resources
	pipeline.SyncMode = readMode
	pipeline.WriteMode = writeMode
	return nil
}

// SimplePipelineGraph builds the complete graph for a simple pipeline,
// keeping the node ids of the graph it was read from.
func SimplePipelineGraph(pipeline Pipeline) PipelineGraph {
	sourceID, sinkID := "source", "sink"
	if pipeline.Graph != nil {
		for _, node := range pipeline.Graph.Nodes {
			switch node.Kind {
			case "source":
				sourceID = node.ID
			case "sink":
				sinkID = node.ID
			}
		}
	}
	readMode := pipeline.SyncMode
	if readMode == SyncModeCDC {
		readMode = ""
	}
	edge := PipelineGraphEdge{From: sourceID, To: sinkID, ReadMode: readMode, WriteMode: pipeline.WriteMode}
	edges := []PipelineGraphEdge{edge}
	if len(pipeline.Resources) > 0 {
		edges = make([]PipelineGraphEdge, 0, len(pipeline.Resources))
		for _, resource := range pipeline.Resources {
			edge.Resource = resource
			edges = append(edges, edge)
		}
	}
	return PipelineGraph{
		Nodes: []PipelineGraphNode{
			{ID: sourceID, Kind: "source", Connection: pipeline.Source.Ref, Config: cloneAnyMap(pipeline.Source.Config), SecretRefs: maps.Clone(pipeline.Source.SecretRefs)},
			{ID: sinkID, Kind: "sink", Connection: pipeline.Sink.Ref, Config: cloneAnyMap(pipeline.Sink.Config), SecretRefs: maps.Clone(pipeline.Sink.SecretRefs)},
		},
		Edges: edges,
	}
}

// ReferencesConnection reports whether the pipeline routes through the named
// connection, consulting the retained graph when there is one.
func (p Pipeline) ReferencesConnection(kind, name string) bool {
	if p.Graph != nil {
		return slices.ContainsFunc(p.Graph.Nodes, func(node PipelineGraphNode) bool {
			return node.Kind == kind && node.Connection == name
		})
	}
	if kind == "source" {
		return p.Source.Ref == name
	}
	return p.Sink.Ref == name
}

func cloneGraph(graph PipelineGraph) *PipelineGraph {
	out := &PipelineGraph{Nodes: slices.Clone(graph.Nodes), Edges: slices.Clone(graph.Edges)}
	for i := range out.Nodes {
		out.Nodes[i].Config = cloneAnyMap(out.Nodes[i].Config)
		out.Nodes[i].SecretRefs = maps.Clone(out.Nodes[i].SecretRefs)
	}
	for i := range out.Edges {
		out.Edges[i].Cursors = slices.Clone(out.Edges[i].Cursors)
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			value = cloneAnyMap(nested)
		}
		out[key] = value
	}
	return out
}
