import { styled } from "@linaria/react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import type { CanvasNode } from "@/pages/pipelines/canvas/types";

const EndpointButton = styled.button`
  min-width: 0;
  padding: 0;

  display: flex;
  align-items: center;
  gap: 8px;

  background-color: transparent;
  border: none;
  text-align: left;

  &:not(:disabled) {
    cursor: pointer;
  }
`;

interface PipelineCanvasPanelResourceEndpointProps {
  nodeId: CanvasNode["id"];
  kind: ConnectorKind;
}

const PipelineCanvasPanelResourceEndpoint = ({
  nodeId,
  kind,
}: PipelineCanvasPanelResourceEndpointProps) => {
  const { selectNode } = usePipelineCanvasSelection();
  const connectionByNodeId = usePipelineCanvasConnections();
  const connection = connectionByNodeId.get(nodeId);

  return (
    <EndpointButton
      type="button"
      onClick={() => selectNode(nodeId)}
      disabled={!connectionByNodeId.has(nodeId)}
    >
      <ConnectorTile
        connector={connection?.connector ?? ""}
        kind={kind}
        size={ConnectorTileSize.MEDIUM}
        isDeleted={!!connection?.deletedAt}
      />
      <Text size={TextSize.BODY_SM} isEllipsis>
        {connection?.name ?? nodeId}
      </Text>
    </EndpointButton>
  );
};

export default PipelineCanvasPanelResourceEndpoint;
