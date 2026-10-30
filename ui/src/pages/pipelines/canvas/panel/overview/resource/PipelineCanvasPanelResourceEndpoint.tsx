import type { FC } from "react";

import { styled } from "@linaria/react";

import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import type { CanvasNode } from "@/pages/pipelines/canvas/types";

const EndpointButton = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  min-width: 0;

  display: flex;
  align-items: center;
  gap: ${t.space[8]};

  text-align: left;

  &:disabled {
    cursor: default;
  }
`;

interface PipelineCanvasPanelResourceEndpointProps {
  nodeId: CanvasNode["id"];
  kind: ConnectorKind;
}

const PipelineCanvasPanelResourceEndpoint: FC<PipelineCanvasPanelResourceEndpointProps> = ({
  nodeId,
  kind,
}) => {
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
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {connection?.name ?? nodeId}
      </Text>
    </EndpointButton>
  );
};

export default PipelineCanvasPanelResourceEndpoint;
