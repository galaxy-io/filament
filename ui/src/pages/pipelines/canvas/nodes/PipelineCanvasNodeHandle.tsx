import type { ComponentProps } from "react";

import { styled } from "@linaria/react";
import { Handle, type Position } from "@xyflow/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_HANDLE_TYPE_MAP } from "@/pages/pipelines/canvas/constants";
import {
  PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE,
  PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE,
} from "@/pages/pipelines/canvas/nodes/constants";
import { usePipelineCanvasReadOnly } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const HandleSlot = styled.div`
  width: ${PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}px;
  height: ${PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}px;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const HandleBase = ({
  $isConnected: _isConnected,
  ...props
}: { $isConnected?: boolean } & ComponentProps<typeof Handle>) => <Handle {...props} />;

const StyledHandle = styled(HandleBase)<{ $isConnected?: boolean }>`
  &.react-flow__handle {
    position: relative;
    top: auto;
    right: auto;
    bottom: auto;
    left: auto;
    transform: none;

    width: ${({ $isConnected }) => ($isConnected ? PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE : PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE)}px;
    height: ${({ $isConnected }) => ($isConnected ? PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE : PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE)}px;

    background-color: ${({ $isConnected }) =>
      $isConnected ? "transparent" : t.color.text.tertiary};
    border: ${({ $isConnected }) =>
      $isConnected ? `2px solid ${t.color.solid.primary.background}` : "none"};
    border-radius: 50%;

    transition:
      width 100ms ease,
      height 100ms ease;
  }

  &.react-flow__handle:hover {
    width: ${PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE}px;
    height: ${PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE}px;
  }
`;

interface PipelineCanvasNodeHandleProps {
  id: string;
  kind: ConnectorKind;
  position: Position;
  isConnected?: boolean;
}

const PipelineCanvasNodeHandle = ({
  id,
  kind,
  position,
  isConnected,
}: PipelineCanvasNodeHandleProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();

  return (
    <HandleSlot>
      <StyledHandle
        id={id}
        type={CONNECTOR_KIND_TO_HANDLE_TYPE_MAP[kind]}
        position={position}
        isConnectable={!isReadOnly}
        $isConnected={isConnected}
      />
    </HandleSlot>
  );
};

export default PipelineCanvasNodeHandle;
