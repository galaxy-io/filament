import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import type { RunResourceStateColumn } from "@/pages/pipelines/history/PipelineHistoryRunInfo";

import { useGetConnectionQuery } from "@/api/queries/connections";

interface PipelineHistoryRunInfoSinkColumnProps {
  runResource: RunResourceStateColumn;
}

const PipelineHistoryRunInfoSinkColumn = ({
  runResource,
}: PipelineHistoryRunInfoSinkColumnProps) => {
  const navigate = useNavigate();

  const { data: sinkConnection } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, {
      id: runResource.sinkConnectionId,
    }),
  });

  const handleConnectorTileClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (sinkConnection?.connection) {
      navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          connectionId: sinkConnection?.connection?.id,
        }),
      });
    }
  };

  const renderContent = () => {
    return (
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} width={200}>
        <ConnectorTile
          connector={sinkConnection?.connection?.connector ?? ""}
          kind={sinkConnection?.connection?.kind}
          size={ConnectorTileSize.SMALL}
          onClick={handleConnectorTileClick}
        />
        <Text size={TextSize.BODY_SM}>{sinkConnection?.connection?.name ?? "—"}</Text>
      </FlexWrapper>
    );
  };

  return renderContent();
};

export default PipelineHistoryRunInfoSinkColumn;
