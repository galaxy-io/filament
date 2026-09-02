package model

import (
	"fmt"
	"slices"
)

// ProjectSimpleGraph fills the CLI's simple pipeline fields from graph when
// doing so loses no graph semantics. The graph is retained even when the
// projection is rejected so view and run operations remain available.
func ProjectSimpleGraph(pipeline *Pipeline, graph PipelineGraph, sourceReplication string) error {
	pipeline.Graph = clonePipelineGraph(&graph)
	var source, sink *PipelineGraphNode
	for index := range graph.Nodes {
		node := &graph.Nodes[index]
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
		return fmt.Errorf("the graph is not a single source-to-sink route")
	}
	pipeline.Source = PipelineNode{
		Ref: source.Connection, Config: cloneAnyMap(source.Config), SecretRefs: cloneStringMap(source.SecretRefs),
	}
	pipeline.Sink = PipelineNode{
		Ref: sink.Connection, Config: cloneAnyMap(sink.Config), SecretRefs: cloneStringMap(sink.SecretRefs),
	}
	if len(graph.Edges) == 0 {
		return fmt.Errorf("the graph has no route")
	}
	seenResources := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.From != source.ID || edge.To != sink.ID {
			return fmt.Errorf("the graph contains more than one source-to-sink route")
		}
		if edge.Selector != "" || len(edge.Cursors) > 0 {
			return fmt.Errorf("the graph uses selectors or cursor overrides")
		}
		if seenResources[edge.Resource] {
			return fmt.Errorf("the graph contains duplicate resource routes")
		}
		seenResources[edge.Resource] = true
		if edge.Resource == "" && len(graph.Edges) != 1 {
			return fmt.Errorf("the graph mixes an all-resource route with resource routes")
		}
		if pipeline.SyncMode == "" {
			pipeline.SyncMode = edge.ReadMode
		} else if pipeline.SyncMode != edge.ReadMode {
			return fmt.Errorf("the graph uses different read modes by resource")
		}
		if pipeline.WriteMode == "" {
			pipeline.WriteMode = edge.WriteMode
		} else if pipeline.WriteMode != edge.WriteMode {
			return fmt.Errorf("the graph uses different write modes by resource")
		}
		if edge.Resource != "" {
			pipeline.Resources = append(pipeline.Resources, edge.Resource)
		}
	}
	if sourceReplication == "cdc" {
		pipeline.SyncMode = "cdc"
	} else if pipeline.SyncMode == "" {
		pipeline.SyncMode = "full"
	}
	return nil
}

// SimplePipelineGraph projects the editable CLI fields back into a complete
// graph while retaining stable node identities from the graph that was read.
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
	if readMode == "cdc" {
		readMode = ""
	}
	edges := []PipelineGraphEdge{{From: sourceID, To: sinkID, ReadMode: readMode, WriteMode: pipeline.WriteMode}}
	if len(pipeline.Resources) > 0 {
		edges = make([]PipelineGraphEdge, 0, len(pipeline.Resources))
		for _, resource := range pipeline.Resources {
			edges = append(edges, PipelineGraphEdge{
				From: sourceID, To: sinkID, Resource: resource, ReadMode: readMode, WriteMode: pipeline.WriteMode,
			})
		}
	}
	return PipelineGraph{
		Nodes: []PipelineGraphNode{
			{ID: sourceID, Kind: "source", Connection: pipeline.Source.Ref, Config: cloneAnyMap(pipeline.Source.Config), SecretRefs: cloneStringMap(pipeline.Source.SecretRefs)},
			{ID: sinkID, Kind: "sink", Connection: pipeline.Sink.Ref, Config: cloneAnyMap(pipeline.Sink.Config), SecretRefs: cloneStringMap(pipeline.Sink.SecretRefs)},
		},
		Edges: edges,
	}
}

// ReferencesConnection reports whether any retained graph node uses name.
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

func clonePipelineGraph(graph *PipelineGraph) *PipelineGraph {
	if graph == nil {
		return nil
	}
	out := &PipelineGraph{
		Nodes: make([]PipelineGraphNode, len(graph.Nodes)),
		Edges: make([]PipelineGraphEdge, len(graph.Edges)),
	}
	for index, node := range graph.Nodes {
		node.Config = cloneAnyMap(node.Config)
		node.SecretRefs = cloneStringMap(node.SecretRefs)
		out.Nodes[index] = node
	}
	for index, edge := range graph.Edges {
		edge.Cursors = append([]ResourceCursor(nil), edge.Cursors...)
		out.Edges[index] = edge
	}
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		if nested, ok := value.(map[string]any); ok {
			value = cloneAnyMap(nested)
		}
		out[key] = value
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
