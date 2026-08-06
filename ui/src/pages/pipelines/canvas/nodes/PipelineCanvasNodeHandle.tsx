import type { ComponentProps } from "react";

import { styled } from "@linaria/react";
import { Handle, type Position } from "@xyflow/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_HANDLE_TYPE_MAP } from "@/pages/pipelines/canvas/constants";
import { PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE } from "@/pages/pipelines/canvas/nodes/constants";

const PORT_SIZE_ACTIVE = 12;
const PORT_SIZE_IDLE = 5;

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
  theme: _theme,
  ...props
}: PropsWithTheme<{ $isConnected?: boolean }> & ComponentProps<typeof Handle>) => (
  <Handle {...props} />
);

const StyledHandle = withTheme(styled(HandleBase)<PropsWithTheme<{ $isConnected?: boolean }>>`
  &.react-flow__handle {
    position: relative;
    top: auto;
    right: auto;
    bottom: auto;
    left: auto;
    transform: none;

    width: ${PORT_SIZE_ACTIVE}px;
    height: ${PORT_SIZE_ACTIVE}px;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: transparent;
    border: none;
    border-radius: 50%;
  }

  &.react-flow__handle::before {
    content: "";
    box-sizing: border-box;

    width: ${({ $isConnected }) => ($isConnected ? PORT_SIZE_ACTIVE : PORT_SIZE_IDLE)}px;
    height: ${({ $isConnected }) => ($isConnected ? PORT_SIZE_ACTIVE : PORT_SIZE_IDLE)}px;

    background-color: ${({ theme, $isConnected }) =>
      $isConnected ? "transparent" : theme.color.text.tertiary};
    border: ${({ theme, $isConnected }) =>
      $isConnected ? `2px solid ${theme.color.background.galaxy}` : "none"};
    border-radius: 50%;

    transition:
      width 100ms ease,
      height 100ms ease;
  }

  &.react-flow__handle:hover::before {
    width: ${PORT_SIZE_ACTIVE}px;
    height: ${PORT_SIZE_ACTIVE}px;
  }
`);

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
}: PipelineCanvasNodeHandleProps) => (
  <HandleSlot>
    <StyledHandle
      id={id}
      type={CONNECTOR_KIND_TO_HANDLE_TYPE_MAP[kind]}
      position={position}
      $isConnected={isConnected}
    />
  </HandleSlot>
);

export default PipelineCanvasNodeHandle;
