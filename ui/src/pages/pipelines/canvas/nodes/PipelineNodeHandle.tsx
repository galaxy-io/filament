import type { ComponentProps } from "react";

import { styled } from "@linaria/react";
import { Handle, type Position } from "@xyflow/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  CONNECTOR_KIND_TO_HANDLE_TYPE_MAP,
  PIPELINE_NODE_HANDLE_SLOT_SIZE,
} from "@/pages/pipelines/canvas/constants";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

// Port sizes from Figma
const PORT_SIZE_ACTIVE = 12;
const PORT_SIZE_IDLE = 5;

// Fixed-size slot that centers the port so layout doesn't shift as it grows
const HandleSlot = styled.div`
  width: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;
  height: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

// Strip styling-only props so styled() doesn't forward them to the DOM through Handle
const HandleBase = ({
  $isConnected: _isConnected,
  theme: _theme,
  ...props
}: PropsWithTheme<{ $isConnected?: boolean }> & ComponentProps<typeof Handle>) => (
  <Handle {...props} />
);

// Override React Flow's absolute positioning so the handle works in the slot system
const StyledHandle = withTheme(styled(HandleBase)<PropsWithTheme<{ $isConnected?: boolean }>>`
  position: relative !important;
  top: auto !important;
  right: auto !important;
  bottom: auto !important;
  left: auto !important;
  transform: none !important;

  width: ${({ $isConnected }) => ($isConnected ? PORT_SIZE_ACTIVE : PORT_SIZE_IDLE)}px !important;
  height: ${({ $isConnected }) => ($isConnected ? PORT_SIZE_ACTIVE : PORT_SIZE_IDLE)}px !important;

  background-color: ${({ theme, $isConnected }) =>
    $isConnected ? "transparent" : theme.color.text.tertiary} !important;
  border: ${({ theme, $isConnected }) =>
    $isConnected ? `2px solid ${theme.color.background.galaxy}` : "none"} !important;
  border-radius: 50% !important;

  transition:
    width 100ms ease,
    height 100ms ease;

  &:hover {
    width: ${PORT_SIZE_ACTIVE}px !important;
    height: ${PORT_SIZE_ACTIVE}px !important;
  }
`);

interface PipelineNodeHandleProps {
  id: string;
  kind: ConnectorKind;
  position: Position;
  isConnected?: boolean;
}

const PipelineNodeHandle = ({ id, kind, position, isConnected }: PipelineNodeHandleProps) => (
  <HandleSlot>
    <StyledHandle
      id={id}
      type={CONNECTOR_KIND_TO_HANDLE_TYPE_MAP[kind]}
      position={position}
      $isConnected={isConnected}
    />
  </HandleSlot>
);

export default PipelineNodeHandle;
