import { styled } from "@linaria/react";
import pluralize from "pluralize";
import { match } from "ts-pattern";

import { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Stepper, { StepperSize, type StepperStep } from "@galaxy-io/dls/navigation/Stepper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import DocsButton from "@/components/DocsButton";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH,
  CREATE_PIPELINE_MODAL_STEP_ORDER,
  CREATE_PIPELINE_MODAL_STEP_TO_DESCRIPTION_MAP,
  CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP,
} from "@/pages/pipelines/components/create/constants";
import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

const SidebarWrapper = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  flex-shrink: 0;

  width: ${CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH}px;

  background-color: ${t.color.background.primary};
  border-right: 0.5px solid ${t.color.border.primary};
`;

const CreatePipelineModalSidebar = () => {
  const {
    step,
    stepIndex,
    sourceConnection,
    sinks,
    executionMode,
    schedule,
    issuesBySink,
    selectedCountBySink,
    isSubmitting,
  } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const hasResourceIssues = Object.values(issuesBySink).some((issues) => issues.length > 0);
  const selectedCount = Object.values(selectedCountBySink).reduce((sum, count) => sum + count, 0);

  const getStepSummary = (item: CreatePipelineModalStep) =>
    match(item)
      .with(CreatePipelineModalStep.CONNECTIONS, () =>
        sourceConnection
          ? `${sourceConnection.name} → ${
              sinks.length === 1 ? sinks[0].connection.name : pluralize("sink", sinks.length, true)
            }`
          : undefined,
      )
      .with(CreatePipelineModalStep.RESOURCES, () => pluralize("resource", selectedCount, true))
      .with(CreatePipelineModalStep.DELIVERY, () => {
        if (executionMode === ExecutionMode.CONTINUOUS) return "Continuous";
        if (!schedule.isEnabled) return "On demand";
        return formatPipelineScheduleSummary(schedule) ?? undefined;
      })
      .with(CreatePipelineModalStep.DETAILS, () => undefined)
      .exhaustive();

  const steps: StepperStep[] = CREATE_PIPELINE_MODAL_STEP_ORDER.map((item, index) => ({
    label: CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP[item],
    description: index < stepIndex ? getStepSummary(item) : undefined,
    isError: item === CreatePipelineModalStep.RESOURCES && index <= stepIndex && hasResourceIssues,
  }));

  const handleStepChange = (index: number) => {
    dispatch({
      type: CreatePipelineModalActionType.GO_TO_STEP,
      payload: CREATE_PIPELINE_MODAL_STEP_ORDER[index],
    });
  };

  return (
    <SidebarWrapper>
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} padding={12} fillWidth>
        <Widget variant={WidgetVariant.SECONDARY}>
          <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12}>
            <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.TERTIARY}>
              Steps
            </Text>
            <Stepper
              steps={steps}
              value={stepIndex}
              onChange={isSubmitting ? undefined : handleStepChange}
              orientation={Orientation.VERTICAL}
              size={StepperSize.SMALL}
              ariaLabel="Pipeline setup"
            />
          </Flex>
        </Widget>
      </Flex>
      <Flex
        alignItems={AlignItems.START}
        direction={FlexDirection.COLUMN}
        gap={12}
        padding={16}
        fillWidth
      >
        <Text isProse weight={TextWeight.REGULAR} variant={TextVariant.TERTIARY}>
          {CREATE_PIPELINE_MODAL_STEP_TO_DESCRIPTION_MAP[step]}
        </Text>
        <DocsButton label="Read the docs" path="/pipelines/create" size={ButtonSize.MEDIUM} />
      </Flex>
    </SidebarWrapper>
  );
};

export default CreatePipelineModalSidebar;
