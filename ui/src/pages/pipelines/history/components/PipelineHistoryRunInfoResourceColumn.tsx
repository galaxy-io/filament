import { create } from "@bufbuild/protobuf";
import { useNavigate } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetRunRequestSchema, type RunResourceState } from "@/gen/ingestion/v1/runs_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import { useGetConnectionQuery } from "@/api/queries/connections";
import { useGetRunQuery } from "@/api/queries/runs";

interface PipelineHistoryRunInfoResourceColumnProps {
  runId: string;
  runResource: RunResourceState;
}

const PipelineHistoryRunInfoResourceColumn = ({
  runId,
  runResource,
}: PipelineHistoryRunInfoResourceColumnProps) => {
  const navigate = useNavigate();

  const { data: runData } = useGetRunQuery({
    input: create(GetRunRequestSchema, { runId }),
  });
  const sourceConnectionId = runData?.snapshot?.run?.sourceConnectionId ?? "";

  const { data: sourceConnection } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: sourceConnectionId }),
    options: { enabled: !!sourceConnectionId },
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

  const renderContent = () => {
    return (
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} width={200}>
        <ConnectorTile
          connector={sourceConnection?.connection?.connector ?? ""}
          kind={sourceConnection?.connection?.kind}
          size={ConnectorTileSize.SMALL}
          onClick={handleConnectorTileClick}
        />
        <FlexItem shrink={0}>
          <Text size={TextSize.BODY_SM}>{sourceConnection?.connection?.name ?? "—"}</Text>
        </FlexItem>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isMonospace>
          /
        </Text>
        <FlexItem shrink={0}>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isMonospace isEllipsis>
            {runResource.resource}
          </Text>
        </FlexItem>
      </FlexWrapper>
    );
  };

  return renderContent();
};

export default PipelineHistoryRunInfoResourceColumn;
