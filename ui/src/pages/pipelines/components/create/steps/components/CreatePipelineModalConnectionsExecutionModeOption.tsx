import type { FC } from "react";

import { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import IconTile from "@/components/IconTile";

import {
  EXECUTION_MODE_TO_DESCRIPTION_MAP,
  EXECUTION_MODE_TO_ICON_MAP,
  EXECUTION_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";

interface CreatePipelineModalConnectionsExecutionModeOptionProps {
  mode: ExecutionMode;
  isSelected: boolean;
  onSelect: () => void;
}

const CreatePipelineModalConnectionsExecutionModeOption: FC<
  CreatePipelineModalConnectionsExecutionModeOptionProps
> = ({ mode, isSelected, onSelect }) => (
  <FlexItem grow={1} basis={0} minWidth={0}>
    <Widget
      isInteractive
      variant={isSelected ? WidgetVariant.SECONDARY : WidgetVariant.PRIMARY}
      isSelected={isSelected}
      onClick={onSelect}
      ariaLabel={EXECUTION_MODE_TO_LABEL_MAP[mode]}
      isFlush
    >
      <Flex alignItems={AlignItems.CENTER} gap={12} padding={[12, 16]} fillWidth>
        <IconTile icon={EXECUTION_MODE_TO_ICON_MAP[mode]} variant={IconVariant.SECONDARY} />
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
          <Text>{EXECUTION_MODE_TO_LABEL_MAP[mode]}</Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            {EXECUTION_MODE_TO_DESCRIPTION_MAP[mode]}
          </Text>
        </Flex>
      </Flex>
    </Widget>
  </FlexItem>
);

export default CreatePipelineModalConnectionsExecutionModeOption;
