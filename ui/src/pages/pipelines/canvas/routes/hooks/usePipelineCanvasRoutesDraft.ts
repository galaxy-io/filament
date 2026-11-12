import { useCallback, useMemo, useState } from "react";

import { addEdge } from "@xyflow/react";

import { ExecutionMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import { canConnectEdge } from "@/pages/pipelines/canvas/graph/rules";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { usePipelineCanvasSourceNodeResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSourceNodeResources";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  type PipelineCanvasRoutesSink,
  usePipelineCanvasRoutesSinks,
} from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesSinks";
import type {
  PipelineCanvasRoutesDraft,
  PipelineCanvasRoutesDraftConfig,
} from "@/pages/pipelines/canvas/routes/types";
import {
  type CanvasNode,
  type PipelineCanvasEdgeData,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";
import { usePipelineExecutionMode } from "@/pages/pipelines/hooks/usePipelineExecutionMode";

export interface PipelineCanvasRoutesDraftState {
  draft: PipelineCanvasRoutesDraft | undefined;
  resource: Resource["name"];
  resourceNames: Resource["name"][] | undefined;
  sinks: PipelineCanvasRoutesSink[];
  sinkCount: number;
  sourceNodeId: CanvasNode["id"] | undefined;
  canOpen: boolean;
  canAdd: boolean;
  open: (resource?: Resource["name"]) => void;
  close: () => void;
  setResource: (resource: Resource["name"]) => void;
  setSinkId: (sinkId: CanvasNode["id"]) => void;
  setConfig: (config: Partial<PipelineCanvasRoutesDraftConfig>) => void;
  add: (config: PipelineCanvasEdgeData, hasReadLevers: boolean) => void;
}

export const usePipelineCanvasRoutesDraft = (): PipelineCanvasRoutesDraftState => {
  const [draft, setDraft] = useState<PipelineCanvasRoutesDraft>();
  const { nodes, edges } = usePipelineCanvasState();
  const { connect, setEdgeConfig, setRouteWriteMode } = usePipelineCanvasActions();
  const { selectResource } = usePipelineCanvasSelection();
  const isReadOnly = usePipelineCanvasReadOnly();
  const isContinuous = usePipelineExecutionMode() === ExecutionMode.CONTINUOUS;
  const allSinks = usePipelineCanvasRoutesSinks();

  const sourceNodeId = nodes
    .filter(isConnectionNode)
    .find((node) => node.type === PipelineCanvasNodeType.SOURCE)?.id;
  const sourceResources = usePipelineCanvasSourceNodeResources();

  const getConnection = useCallback(
    (resource: Resource["name"], sinkId: CanvasNode["id"]) => ({
      source: sourceNodeId ?? "",
      sourceHandle: resource,
      target: sinkId,
      targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
    }),
    [sourceNodeId],
  );
  const canRoute = useCallback(
    (resource: Resource["name"], sinkId: CanvasNode["id"]) =>
      resource !== "" && canConnectEdge(getConnection(resource, sinkId), edges),
    [getConnection, edges],
  );

  const discoveredNames = useMemo(
    () => (isContinuous ? [] : sourceResources),
    [isContinuous, sourceResources],
  );
  const resourceNames = useMemo(() => {
    if (isContinuous || !draft) return undefined;
    return draft.isResourceLocked
      ? discoveredNames
      : discoveredNames.filter((name) => canRoute(name, draft.sinkId));
  }, [isContinuous, discoveredNames, draft, canRoute]);

  const resource = draft?.resource ?? resourceNames?.[0] ?? "";
  const sinks = useMemo(
    () =>
      draft?.isResourceLocked
        ? allSinks.filter((sink) => canRoute(resource, sink.nodeId))
        : allSinks,
    [draft?.isResourceLocked, allSinks, canRoute, resource],
  );

  const open = useCallback(
    (lockedResource?: Resource["name"]) => {
      const hasRoutableResource = (sinkId: CanvasNode["id"]) =>
        lockedResource === undefined
          ? discoveredNames.some((name) => canRoute(name, sinkId))
          : canRoute(lockedResource, sinkId);
      const sinkId =
        allSinks.find((sink) => hasRoutableResource(sink.nodeId))?.nodeId ??
        (lockedResource === undefined ? allSinks[0]?.nodeId : undefined);
      if (sinkId === undefined) return;
      setDraft({
        resource: lockedResource,
        sinkId,
        isResourceLocked: lockedResource !== undefined,
        readMode: undefined,
        writeMode: undefined,
        cursor: undefined,
      });
    },
    [allSinks, discoveredNames, canRoute],
  );
  const close = useCallback(() => setDraft(undefined), []);
  const setResource = useCallback(
    (nextResource: Resource["name"]) =>
      setDraft((prev) => prev && { ...prev, resource: nextResource, cursor: undefined }),
    [],
  );
  const setConfig = useCallback(
    (config: Partial<PipelineCanvasRoutesDraftConfig>) =>
      setDraft((prev) => prev && { ...prev, ...config }),
    [],
  );
  const setSinkId = useCallback(
    (sinkId: CanvasNode["id"]) =>
      setDraft(
        (prev) =>
          prev && {
            ...prev,
            sinkId,
            resource:
              prev.isResourceLocked ||
              (prev.resource !== undefined && canRoute(prev.resource, sinkId))
                ? prev.resource
                : undefined,
          },
      ),
    [canRoute],
  );

  const canAdd = draft !== undefined && canRoute(resource, draft.sinkId);
  const add = useCallback(
    (config: PipelineCanvasEdgeData, hasReadLevers: boolean) => {
      if (!draft || !canAdd) return;
      const connection = getConnection(resource, draft.sinkId);
      const [edge] = addEdge({ ...connection, type: PIPELINE_CANVAS_EDGE_TYPE }, []);
      connect([connection]);
      setEdgeConfig(edge.id, config);
      if (!hasReadLevers && config.writeMode !== WriteMode.UNSPECIFIED)
        setRouteWriteMode(connection.source, connection.target, config.writeMode);
      setDraft(undefined);
      selectResource(edge.id);
    },
    [
      draft,
      canAdd,
      getConnection,
      resource,
      connect,
      setEdgeConfig,
      setRouteWriteMode,
      selectResource,
    ],
  );

  return {
    draft,
    resource,
    resourceNames,
    sinks,
    sinkCount: allSinks.length,
    sourceNodeId,
    canOpen: !isReadOnly && sourceNodeId !== undefined && allSinks.length > 0,
    canAdd,
    open,
    close,
    setResource,
    setSinkId,
    setConfig,
    add,
  };
};
