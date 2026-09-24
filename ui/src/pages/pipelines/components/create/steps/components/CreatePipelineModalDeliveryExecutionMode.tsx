import { styled } from "@linaria/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_EXECUTION_MODE_ICON_SIZE,
  CREATE_PIPELINE_MODAL_EXECUTION_MODE_TILE_SIZE,
  EXECUTION_MODE_TO_DESCRIPTION_MAP,
  EXECUTION_MODE_TO_ICON_MAP,
  EXECUTION_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import { PIPELINE_EXECUTION_MODES } from "@/pages/pipelines/constants";

import { NOOP } from "@/constants";

const IconTile = withTheme(styled.div<PropsWithTheme>`
  width: ${CREATE_PIPELINE_MODAL_EXECUTION_MODE_TILE_SIZE}px;
  height: ${CREATE_PIPELINE_MODAL_EXECUTION_MODE_TILE_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  border: 0.5px solid ${({ theme }) => theme.color.border.tertiary};
  border-radius: 6px;
`);

interface CreatePipelineModalDeliveryExecutionModeOptionProps {
  mode: ExecutionMode;
  isSelected: boolean;
  isDisabled: boolean;
  onSelect: () => void;
}

const CreatePipelineModalDeliveryExecutionModeOption = ({
  mode,
  isSelected,
  isDisabled,
  onSelect,
}: CreatePipelineModalDeliveryExecutionModeOptionProps) => (
  <FlexItem grow={1} basis={0} minWidth={0}>
    <Widget
      variant={
        isDisabled
          ? WidgetVariant.TERTIARY
          : isSelected
            ? WidgetVariant.SECONDARY
            : WidgetVariant.PRIMARY
      }
      isSelected={isSelected}
      onClick={isDisabled ? undefined : onSelect}
      noHover={isDisabled}
      padding="12px 16px"
      fillWidth
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={12} fillWidth>
        <RadioInput isSelected={isSelected} isDisabled={isDisabled} onChange={NOOP} />
        <IconTile>
          <Icon
            component={EXECUTION_MODE_TO_ICON_MAP[mode]}
            variant={isDisabled ? IconVariant.DISABLED : IconVariant.SECONDARY}
            size={CREATE_PIPELINE_MODAL_EXECUTION_MODE_ICON_SIZE}
          />
        </IconTile>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
          <Text variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}>
            {EXECUTION_MODE_TO_LABEL_MAP[mode]}
          </Text>
          <Text
            size={TextSize.BODY_SM}
            variant={isDisabled ? TextVariant.DISABLED : TextVariant.SECONDARY}
          >
            {EXECUTION_MODE_TO_DESCRIPTION_MAP[mode]}
          </Text>
        </FlexWrapper>
      </FlexWrapper>
    </Widget>
  </FlexItem>
);

const CreatePipelineModalDeliveryExecutionMode = () => {
  const { executionMode, supportedExecutionModes } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <FlexWrapper alignItems={AlignItems.STRETCH} gap={12} fillWidth>
      {PIPELINE_EXECUTION_MODES.map((mode) => (
        <CreatePipelineModalDeliveryExecutionModeOption
          key={mode}
          mode={mode}
          isSelected={executionMode === mode}
          isDisabled={!supportedExecutionModes.includes(mode)}
          onSelect={() =>
            dispatch({ type: CreatePipelineModalActionType.SET_EXECUTION_MODE, payload: mode })
          }
        />
      ))}
    </FlexWrapper>
  );
};

export default CreatePipelineModalDeliveryExecutionMode;
