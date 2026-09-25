import { styled } from "@linaria/react";
import { FunctionIcon } from "@phosphor-icons/react";
import { Position } from "@xyflow/react";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";
import { LayoutSize } from "@/layouts/types";

import { PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

const TableRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  min-width: 0;
`;

const TableListShimmer = () => (
  <>
    {Array.from({ length: PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT }).map((_, index) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
      <TextShimmer key={index} height={16} width="100%" />
    ))}
  </>
);

const TableListRow = ({ table }: { table: PipelineCanvasNodeTableInfo }) => (
  <TableRow>
    <Text
      size={TextSize.BODY_SM}
      variant={table.isConnected ? TextVariant.PRIMARY : TextVariant.TERTIARY}
      isMonospace
      isEllipsis
    >
      {table.name}
    </Text>
    {table.hasTransform && (
      <Icon
        component={FunctionIcon}
        size={12}
        variant={table.isInvalid ? IconVariant.ERROR : IconVariant.SECONDARY}
      />
    )}
    <PipelineCanvasNodeHandle
      id={table.name}
      kind={ConnectorKind.SOURCE}
      position={Position.Right}
      isConnected={table.isConnected}
      isInvalid={table.isInvalid}
    />
  </TableRow>
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
        <ErrorLayout size={LayoutSize.SMALL} message="Failed to load resources" error={error} />
      </FlexWrapper>
    );
  }

  if (!tables.length) {
    return (
      <FlexWrapper padding={"20px 16px"} fillWidth>
        <EmptyLayout size={LayoutSize.SMALL} message="No tables match your search" />
      </FlexWrapper>
    );
  }

  return tables.map((table) => <TableListRow key={table.name} table={table} />);
};

export default PipelineCanvasNodeSourceIslandTableList;
