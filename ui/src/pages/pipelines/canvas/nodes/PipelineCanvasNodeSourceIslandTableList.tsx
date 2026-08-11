import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { Position } from "@xyflow/react";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import { PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

const TableRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
`;

const TableListShimmer = () => (
  <>
    {Array.from({ length: PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT }).map((_, index) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
      <TextShimmer key={index} height={16} width="100%" />
    ))}
  </>
);

interface PipelineCanvasNodeSourceIslandTableListProps {
  tables: PipelineCanvasNodeTableInfo[];
  error?: Error | null;
  isLoading?: boolean;
}

const PipelineCanvasNodeSourceIslandTableList = ({
  tables,
  error,
  isLoading = false,
}: PipelineCanvasNodeSourceIslandTableListProps) => {
  if (isLoading) {
    return <TableListShimmer />;
  }

  if (error) {
    return (
      <FlexWrapper padding={"20px 16px"} fillWidth>
        <ErrorLayout
          icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
          message="Failed to load resources"
          error={error}
        />
      </FlexWrapper>
    );
  }

  if (!tables.length) {
    return (
      <FlexWrapper padding={"20px 16px"} fillWidth>
        <EmptyLayout message="No tables match your search" />
      </FlexWrapper>
    );
  }

  return tables.map((table) => (
    <TableRow key={table.name}>
      <Text
        size={TextSize.BODY_SM}
        variant={table.isConnected ? TextVariant.SECONDARY : TextVariant.TERTIARY}
        isMonospace
      >
        {table.name}
      </Text>
      <PipelineCanvasNodeHandle
        id={table.name}
        kind={ConnectorKind.SOURCE}
        position={Position.Right}
        isConnected={table.isConnected}
      />
    </TableRow>
  ));
};

export default PipelineCanvasNodeSourceIslandTableList;
