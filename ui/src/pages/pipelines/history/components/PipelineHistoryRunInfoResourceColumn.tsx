import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import type { RunResourceStateColumn } from "@/pages/pipelines/history/PipelineHistoryRunInfo";

import { useGetConnectionQuery } from "@/api/queries/connections";

interface PipelineHistoryRunInfoResourceColumnProps {
  runResource: RunResourceStateColumn;
}

const PipelineHistoryRunInfoResourceColumn = ({
  runResource,
}: PipelineHistoryRunInfoResourceColumnProps) => {
  const navigate = useNavigate();

  const { data: sourceConnection } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, {
      id: runResource.sourceConnectionId,
    }),
  });

  const handleConnectorTileClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (sourceConnection?.connection) {
      navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          connectionId: sourceConnection?.connection?.id,
        }),
      });
    }
  };

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      <ConnectorTile
        connector={sourceConnection?.connection?.connector ?? ""}
        size={ConnectorTileSize.SMALL}
        onClick={handleConnectorTileClick}
      />
      <Text size={TextSize.BODY_SM} variant={TextVariant.PRIMARY}>
        {sourceConnection?.connection?.name ?? "—"}
      </Text>
      <Text size={TextSize.BODY_SM} isMonospace>
        /
      </Text>
      <Text size={TextSize.BODY_SM} isMonospace isEllipsis>
        {runResource.resource}
      </Text>
    </FlexWrapper>
  );
};

export default PipelineHistoryRunInfoResourceColumn;
