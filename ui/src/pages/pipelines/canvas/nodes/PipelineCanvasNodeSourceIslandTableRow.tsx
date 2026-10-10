import type { FC } from "react";

import { Position } from "@xyflow/react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";
import PipelineTransformFieldsMarker from "@/pages/pipelines/components/transform/PipelineTransformFieldsMarker";

interface PipelineCanvasNodeSourceIslandTableRowProps {
  table: PipelineCanvasNodeTableInfo;
}

const PipelineCanvasNodeSourceIslandTableRow: FC<PipelineCanvasNodeSourceIslandTableRowProps> = ({
  table,
}) => (
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

export default PipelineCanvasNodeSourceIslandTableRow;
