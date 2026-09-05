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

// ModeMixed is the summary sync or write mode when a graph's routes disagree.
const ModeMixed = "mixed"

// ProjectSimpleGraph fills the simple pipeline fields from graph. Source, sink,
// resources, and modes are summarized whenever the graph is one source and one
// sink; the returned error is why the simple editor cannot round-trip it
// (selectors, cursor overrides, modes that differ by resource), which the
// caller records as the edit block. The graph is retained either way.
func ProjectSimpleGraph(pipeline *Pipeline, graph PipelineGraph, sourceReplication string) error {
	pipeline.Graph = cloneGraph(graph)
	source, sink, err := simpleRouteNodes(graph)
	if err != nil {
		return err
	}
	pipeline.Source = PipelineNode{Ref: source.Connection, Config: CloneConfig(source.Config), SecretRefs: maps.Clone(source.SecretRefs)}
	pipeline.Sink = PipelineNode{Ref: sink.Connection, Config: CloneConfig(sink.Config), SecretRefs: maps.Clone(sink.SecretRefs)}
	if len(graph.Edges) == 0 {
		return fmt.Errorf("no route")
	}
	var blocked error
	block := func(format string, args ...any) {
		if blocked == nil {
			blocked = fmt.Errorf(format, args...)
		}
	}
	readMode, writeMode := graph.Edges[0].ReadMode, graph.Edges[0].WriteMode
	var resources []string
	all := false
	seen := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.From != source.ID || edge.To != sink.ID {
			block("more than one source-to-sink route")
		}
		if edge.Selector != "" || len(edge.Cursors) > 0 {
			block("selectors or cursor overrides in use")
		}
		switch {
		case seen[edge.Resource]:
			block("duplicate route for resource %q", edge.Resource)
		case edge.Resource == "":
			all = true
			if len(graph.Edges) != 1 {
				block("all-resource route mixed with resource routes")
			}
		default:
			resources = append(resources, edge.Resource)
		}
		seen[edge.Resource] = true
		if edge.ReadMode != readMode {
			readMode = ModeMixed
			block("read mode differs by resource")
		}
		if edge.WriteMode != writeMode {
			writeMode = ModeMixed
			block("write mode differs by resource")
		}
	}
	switch {
	case sourceReplication == SyncModeCDC:
		if readMode != "" {
			block("read mode set on a CDC source")
		}
		readMode = SyncModeCDC
	case readMode == "":
		readMode = "full"
	}
	if all {
		resources = nil
	}
	pipeline.Resources = resources
	pipeline.SyncMode = readMode
	pipeline.WriteMode = writeMode
	return blocked
}

// simpleRouteNodes returns the graph's one source and one sink node.
func simpleRouteNodes(graph PipelineGraph) (source, sink *PipelineGraphNode, err error) {
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		switch node.Kind {
		case "source":
			if source != nil {
				return nil, nil, fmt.Errorf("multiple source nodes")
			}
			source = node
		case "sink":
			if sink != nil {
				return nil, nil, fmt.Errorf("multiple sink nodes")
			}
			sink = node
		default:
			return nil, nil, fmt.Errorf("node %q has unknown kind %q", node.ID, node.Kind)
		}
	}
	if len(graph.Nodes) != 2 || source == nil || sink == nil {
		return nil, nil, fmt.Errorf("not a single source-to-sink route")
	}
	return source, sink, nil
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
			{ID: sourceID, Kind: "source", Connection: pipeline.Source.Ref, Config: CloneConfig(pipeline.Source.Config), SecretRefs: maps.Clone(pipeline.Source.SecretRefs)},
			{ID: sinkID, Kind: "sink", Connection: pipeline.Sink.Ref, Config: CloneConfig(pipeline.Sink.Config), SecretRefs: maps.Clone(pipeline.Sink.SecretRefs)},
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
		out.Nodes[i].Config = CloneConfig(out.Nodes[i].Config)
		out.Nodes[i].SecretRefs = maps.Clone(out.Nodes[i].SecretRefs)
	}
	for i := range out.Edges {
		out.Edges[i].Cursors = slices.Clone(out.Edges[i].Cursors)
	}
	return out
}
