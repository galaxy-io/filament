import type { ComponentProps, FC } from "react";

import { styled } from "@linaria/react";
import { Handle, type Position } from "@xyflow/react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_HANDLE_TYPE_MAP } from "@/pages/pipelines/canvas/constants";
import {
  PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE,
  PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE,
} from "@/pages/pipelines/canvas/nodes/constants";
import { usePipelineCanvasReadOnly } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

type HandleFlags = { $isConnected?: boolean; $isInvalid?: boolean };

const HandleBase: FC<HandleFlags & ComponentProps<typeof Handle>> = ({
  $isConnected: _isConnected,
  $isInvalid: _isInvalid,
  ...props
}) => <Handle {...props} />;

const StyledHandle = styled(HandleBase)<HandleFlags>`
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
    border: ${({ $isConnected, $isInvalid }) =>
      $isConnected
        ? `2px solid ${$isInvalid ? t.color.text.error : t.color.solid.primary.background}`
        : "none"};
    border-radius: ${t.radius.pill};

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
  isInvalid?: boolean;
}

const PipelineCanvasNodeHandle: FC<PipelineCanvasNodeHandleProps> = ({
  id,
  kind,
  position,
  isConnected,
  isInvalid,
}) => {
  const isReadOnly = usePipelineCanvasReadOnly();

  return (
    <Flex
      width={PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}
      height={PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}
      shrink={0}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
    >
      <StyledHandle
        id={id}
        type={CONNECTOR_KIND_TO_HANDLE_TYPE_MAP[kind]}
        position={position}
        isConnectable={!isReadOnly}
        $isConnected={isConnected}
        $isInvalid={isInvalid}
      />
    </Flex>
  );
};

export default PipelineCanvasNodeHandle;
