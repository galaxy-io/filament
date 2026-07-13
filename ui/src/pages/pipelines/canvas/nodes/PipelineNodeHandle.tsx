import { Handle, Position } from "@xyflow/react";
import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_HANDLE_TYPE_MAP } from "@/pages/pipelines/canvas/constants";

// Port sizes from Figma
const PORT_SIZE_ACTIVE = 12;
const PORT_SIZE_IDLE = 5;

// Override React Flow's absolute positioning so handle works in slot system
const StyledHandle = withTheme(styled(Handle)<PropsWithTheme<{ $isConnected?: boolean }>>`
  position: relative !important;
  top: auto !important;
  right: auto !important;
  bottom: auto !important;
  left: auto !important;
  transform: none !important;

  width: ${({ $isConnected }) =>
    $isConnected ? PORT_SIZE_ACTIVE : PORT_SIZE_IDLE}px !important;
  height: ${({ $isConnected }) =>
    $isConnected ? PORT_SIZE_ACTIVE : PORT_SIZE_IDLE}px !important;

  background-color: ${({ theme, $isConnected }) =>
    $isConnected ? "transparent" : theme.color.text.tertiary} !important;
  border: ${({ theme, $isConnected }) =>
    $isConnected ? `2px solid ${theme.color.background.galaxy}` : "none"} !important;
  border-radius: 50% !important;

  transition: all 100ms ease;

  &:hover {
    width: ${PORT_SIZE_ACTIVE}px !important;
    height: ${PORT_SIZE_ACTIVE}px !important;
    background-color: ${({ theme, $isConnected }) =>
      $isConnected ? "transparent" : theme.color.text.tertiary} !important;
    border: ${({ theme, $isConnected }) =>
      $isConnected ? `2px solid ${theme.color.background.galaxy}` : "none"} !important;
  }
`);

interface PipelineNodeHandleProps {
  id: string;
  kind: ConnectorKind;
  position: Position;
  isConnected?: boolean;
}

const PipelineNodeHandle = ({ id, kind, position, isConnected }: PipelineNodeHandleProps) => {
  const handleType = CONNECTOR_KIND_TO_HANDLE_TYPE_MAP[kind];

  return (
    <StyledHandle
      id={id}
      type={handleType}
      position={position}
      $isConnected={isConnected}
    />
  );
};

export default PipelineNodeHandle;
