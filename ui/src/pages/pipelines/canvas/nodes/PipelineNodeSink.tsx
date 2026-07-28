import { memo } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_NODE_SINK_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";
import type { PipelineNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/providers/canvas/actions";
import {
  usePipelineCanvasDispatch,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const PipelineNodeSink = memo(({ id, data, selected }: PipelineNodeSinkProps) => {
  const dispatch = usePipelineCanvasDispatch();
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });

  const handleDelete = () => {
    dispatch({ type: PipelineCanvasActionType.REMOVE_NODE, payload: id });
  };

  return (
    <PipelineNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SINK}
      handleId={PIPELINE_NODE_SINK_HANDLE_ID}
      isConnected={connections.length > 0}
      isSelected={selected}
      onDelete={isReadOnly ? undefined : handleDelete}
    />
  );
});

PipelineNodeSink.displayName = "PipelineNodeSink";

export default PipelineNodeSink;
