import type { FC } from "react";

import { styled } from "@linaria/react";
import { Handle, type Position } from "@xyflow/react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_HANDLE_TYPE_MAP } from "@/pages/pipelines/canvas/constants";
import {
  PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_CANVAS_NODE_PORT_BORDER_WIDTH,
  PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE,
  PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE,
} from "@/pages/pipelines/canvas/nodes/constants";
import { usePipelineCanvasReadOnly } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const StyledHandle = styled(Handle)`
  &.react-flow__handle {
    position: relative;
    top: auto;
    right: auto;
    bottom: auto;
    left: auto;
    transform: none;

    width: ${PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE}px;
    height: ${PIPELINE_CANVAS_NODE_PORT_SIZE_IDLE}px;

    background-color: ${t.color.text.tertiary};
    border: none;
    border-radius: ${t.radius.pill};

    transition:
      width ${t.duration.fast},
      height ${t.duration.fast};
  }

  &.react-flow__handle[data-connected="true"] {
    width: ${PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE}px;
    height: ${PIPELINE_CANVAS_NODE_PORT_SIZE_ACTIVE}px;

    background-color: transparent;
    border: ${PIPELINE_CANVAS_NODE_PORT_BORDER_WIDTH}px solid ${t.color.solid.primary.background};
  }

  &.react-flow__handle[data-connected="true"][data-invalid="true"] {
    border-color: ${t.color.border.error};
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
        data-connected={!!isConnected}
        data-invalid={!!isInvalid}
      />
    </Flex>
  );
};

export default PipelineCanvasNodeHandle;
