import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { type Connection, GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { RunResourceState } from "@/gen/ingestion/v1/runs_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import { useGetConnectionQuery } from "@/api/queries/connections";

interface PipelineHistoryRunInfoConnectionColumnProps {
  connectionId: Connection["id"];
  resourceName?: RunResourceState["resourceName"];
}

const PipelineHistoryRunInfoConnectionColumn = ({
  connectionId,
  resourceName,
}: PipelineHistoryRunInfoConnectionColumnProps) => {
  const navigate = useNavigate();

  const { data: connectionData } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: connectionId }),
    options: { enabled: !!connectionId },
  });

  const handleConnectorTileClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (connectionData?.connection) {
      navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          connectionId: connectionData?.connection?.id,
        }),
      });
    }
  };

  const renderContent = () => {
    return (
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} width={200}>
        <ConnectorTile
          connector={connectionData?.connection?.connector ?? ""}
          kind={connectionData?.connection?.kind}
          size={ConnectorTileSize.SMALL}
          onClick={handleConnectorTileClick}
        />
        <FlexItem shrink={0}>
          <Text size={TextSize.BODY_SM}>{connectionData?.connection?.name ?? "—"}</Text>
        </FlexItem>
        {resourceName && (
          <>
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isMonospace>
              /
            </Text>
            <FlexItem shrink={0}>
              <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isMonospace isEllipsis>
                {resourceName}
              </Text>
            </FlexItem>
          </>
        )}
      </FlexWrapper>
    );
  };

  return renderContent();
};

export default PipelineHistoryRunInfoConnectionColumn;
