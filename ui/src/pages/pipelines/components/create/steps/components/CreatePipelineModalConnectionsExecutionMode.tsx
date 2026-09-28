import { InfoIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";
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

const CreatePipelineModalConnectionsExecutionModeOption = ({
  mode,
  isSelected,
  onSelect,
}: CreatePipelineModalConnectionsExecutionModeOptionProps) => (
  <FlexItem grow={1} basis={0} minWidth={0}>
    <Widget
      variant={isSelected ? WidgetVariant.SECONDARY : WidgetVariant.PRIMARY}
      isSelected={isSelected}
      onClick={onSelect}
      padding="12px 16px"
      fillWidth
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={12} fillWidth>
        <RadioInput isSelected={isSelected} onChange={NOOP} />
        <IconTile icon={EXECUTION_MODE_TO_ICON_MAP[mode]} variant={IconVariant.SECONDARY} />
        <FlexWrapper direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
          <Text>{EXECUTION_MODE_TO_LABEL_MAP[mode]}</Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            {EXECUTION_MODE_TO_DESCRIPTION_MAP[mode]}
          </Text>
        </FlexWrapper>
      </FlexWrapper>
    </Widget>
  </FlexItem>
);

const CreatePipelineModalConnectionsExecutionMode = () => {
  const { executionMode } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Widget variant={WidgetVariant.PRIMARY} fillWidth>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={4}>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.TERTIARY}>
            Execution type
          </Text>
          <Tooltip
            position={TooltipPosition.TOP_START}
            body={
              <FlexWrapper direction={FlexDirection.COLUMN} gap={8} maxWidth={320}>
                {PIPELINE_EXECUTION_MODES.map((mode) => (
                  <FlexWrapper key={mode} direction={FlexDirection.COLUMN} gap={2}>
                    <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
                      {EXECUTION_MODE_TO_LABEL_MAP[mode]}
                    </Text>
                    <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                      {EXECUTION_MODE_TO_DETAILS_MAP[mode]}
                    </Text>
                  </FlexWrapper>
                ))}
              </FlexWrapper>
            }
          >
            <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
          </Tooltip>
        </FlexWrapper>
        <FlexWrapper alignItems={AlignItems.STRETCH} gap={12} fillWidth>
          {PIPELINE_EXECUTION_MODES.map((mode) => (
            <CreatePipelineModalConnectionsExecutionModeOption
              key={mode}
              mode={mode}
              isSelected={executionMode === mode}
              onSelect={() =>
                dispatch({ type: CreatePipelineModalActionType.SET_EXECUTION_MODE, payload: mode })
              }
            />
          ))}
        </FlexWrapper>
      </FlexWrapper>
    </Widget>
  );
};

export default CreatePipelineModalConnectionsExecutionMode;
