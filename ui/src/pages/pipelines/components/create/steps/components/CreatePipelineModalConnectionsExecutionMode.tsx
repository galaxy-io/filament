import type { FC } from "react";

import { InfoIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Placement } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import IconTile from "@/components/IconTile";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  EXECUTION_MODE_TO_DESCRIPTION_MAP,
  EXECUTION_MODE_TO_DETAILS_MAP,
  EXECUTION_MODE_TO_ICON_MAP,
  EXECUTION_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import { PIPELINE_EXECUTION_MODES } from "@/pages/pipelines/constants";

import { NOOP } from "@/constants";

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
      isFlush
    >
      <Flex alignItems={AlignItems.CENTER} gap={12} padding={[12, 16]} fillWidth>
        <RadioInput isSelected={isSelected} onChange={NOOP} />
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

const CreatePipelineModalConnectionsExecutionMode: FC = () => {
  const { executionMode } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Box fillWidth>
      <Widget variant={WidgetVariant.PRIMARY}>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <Flex alignItems={AlignItems.CENTER} gap={4}>
            <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.TERTIARY}>
              Execution type
            </Text>
            <Tooltip
              placement={Placement.TOP_START}
              body={
                <Flex
                  alignItems={AlignItems.START}
                  direction={FlexDirection.COLUMN}
                  gap={8}
                  maxWidth={320}
                >
                  {PIPELINE_EXECUTION_MODES.map((mode) => (
                    <Flex
                      alignItems={AlignItems.START}
                      key={mode}
                      direction={FlexDirection.COLUMN}
                      gap={2}
                    >
                      <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
                        {EXECUTION_MODE_TO_LABEL_MAP[mode]}
                      </Text>
                      <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                        {EXECUTION_MODE_TO_DETAILS_MAP[mode]}
                      </Text>
                    </Flex>
                  ))}
                </Flex>
              }
            >
              <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
            </Tooltip>
          </Flex>
          <Flex alignItems={AlignItems.STRETCH} gap={12} fillWidth>
            {PIPELINE_EXECUTION_MODES.map((mode) => (
              <CreatePipelineModalConnectionsExecutionModeOption
                key={mode}
                mode={mode}
                isSelected={executionMode === mode}
                onSelect={() =>
                  dispatch({
                    type: CreatePipelineModalActionType.SET_EXECUTION_MODE,
                    payload: mode,
                  })
                }
              />
            ))}
          </Flex>
        </Flex>
      </Widget>
    </Box>
  );
};

export default CreatePipelineModalConnectionsExecutionMode;
