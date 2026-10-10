import type { FC } from "react";

import { Position } from "@xyflow/react";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";
import PipelineTransformFieldsMarker from "@/pages/pipelines/components/transform/PipelineTransformFieldsMarker";

import { IS_DEBUG } from "@/constants";

const TableListShimmer: FC = () => (
  <>
    {Array.from({ length: PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT }).map((_, index) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
      <Box key={index} width="100%">
        <Skeleton />
      </Box>
    ))}
  </>
);

const TableListRow: FC<{ table: PipelineCanvasNodeTableInfo }> = ({ table }) => (
  <Flex
    alignItems={AlignItems.CENTER}
    justifyContent={JustifyContent.SPACE_BETWEEN}
    gap={4}
    minWidth={0}
  >
    <FlexItem grow={1} minWidth={0}>
      <Text
        size={TextSize.BODY_SM}
        variant={table.isConnected ? TextVariant.PRIMARY : TextVariant.TERTIARY}
        family={FontFamily.MONO}
        lineClamp={1}
      >
        {table.name}
      </Text>
    </FlexItem>
    {table.hasTransform && <PipelineTransformFieldsMarker isInvalid={table.isInvalid} />}
    <PipelineCanvasNodeHandle
      id={table.name}
      kind={ConnectorKind.SOURCE}
      position={Position.Right}
      isConnected={table.isConnected}
      isInvalid={table.isInvalid}
    />
  </Flex>
);

interface PipelineCanvasNodeSourceIslandTableListProps {
  tables: PipelineCanvasNodeTableInfo[];
  error?: Error | null;
  isLoading?: boolean;
}

const PipelineCanvasNodeSourceIslandTableList: FC<PipelineCanvasNodeSourceIslandTableListProps> = ({
  tables,
  error,
  isLoading = false,
}) => {
  if (isLoading) {
    return <TableListShimmer />;
  }

  if (error) {
    return (
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <ErrorLayout
          size={ErrorLayoutSize.SMALL}
          header="Failed to load resources"
          detail={IS_DEBUG ? error.message : undefined}
        />
      </Flex>
    );
  }

  if (!tables.length) {
    return (
      <Flex alignItems={AlignItems.START} padding={16} fillWidth>
        <EmptyLayout size={EmptyLayoutSize.SMALL} header="No tables match your search" />
      </Flex>
    );
  }

  return tables.map((table) => <TableListRow key={table.name} table={table} />);
};

export default PipelineCanvasNodeSourceIslandTableList;
