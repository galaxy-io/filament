import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ConnectorKind, ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import {
  PipelineEdgeSchema,
  PipelineGraphSchema,
  PipelineNodeSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasEdgeResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeResources";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/types";
import { getEdgeModeOptions } from "@/pages/pipelines/canvas/utils";
import { usePipelineExecutionMode } from "@/pages/pipelines/hooks/usePipelineExecutionMode";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const usePipelineCanvasPanelResourceOptions = (edge: CanvasEdge) => {
  const executionMode = usePipelineExecutionMode();
  const { nodes } = usePipelineCanvasState();
  const configFor = (id: string) => {
    const node = nodes.find((node) => node.id === id);
    return node && isConnectionNode(node) ? node.data.config : undefined;
  };
  const sourceConfig = configFor(edge.source);
  const sinkConfig = configFor(edge.target);
  const connectionByNodeId = usePipelineCanvasConnections();
  const sourceConnection = connectionByNodeId.get(edge.source);
  const sinkConnection = connectionByNodeId.get(edge.target);
  const resources = usePipelineCanvasEdgeResources(edge);
  const { edgeResource, hasReadLevers, isLoadingColumns } = resources;

  const validationInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        executionMode,
        graph: create(PipelineGraphSchema, {
          nodes:
            sourceConnection && sinkConnection
              ? [
                  create(PipelineNodeSchema, {
                    id: edge.source,
                    kind: ConnectorKind.SOURCE,
                    connectionId: sourceConnection.id,
                    config: sourceConfig,
                  }),
                  create(PipelineNodeSchema, {
                    id: edge.target,
                    kind: ConnectorKind.SINK,
                    connectionId: sinkConnection.id,
                    config: sinkConfig,
                  }),
                ]
              : [],
          edges: [
            create(PipelineEdgeSchema, {
              fromNode: edge.source,
              toNode: edge.target,
              resource: edgeResource,
              destinationResource: edge.data?.destinationResource,
              readMode: edge.data?.readMode ?? ReadMode.UNSPECIFIED,
              writeMode: edge.data?.writeMode ?? WriteMode.UNSPECIFIED,
              cursors: edge.data?.cursors ?? [],
            }),
          ],
        }),
      }),
    [sourceConnection, sinkConnection, edge, edgeResource, executionMode, sourceConfig, sinkConfig],
  );

  const { data: validation, isLoading: isLoadingValidation } = useValidatePipelineQuery({
    input: validationInput,
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: !!sourceConnection && !!sinkConnection,
    },
  });

  const verdict = validation?.edges[0];
  const modeOptions = useMemo(
    () => getEdgeModeOptions(verdict, hasReadLevers),
    [verdict, hasReadLevers],
  );

  return {
    ...resources,
    ...modeOptions,
    verdict,
    isLoading: isLoadingValidation || isLoadingColumns,
  };
};
