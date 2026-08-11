import { styled } from "@linaria/react";

import { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon from "@galaxy-io/dls/icons/Icon";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import DocsButton from "@/components/DocsButton";

import BaseHeader from "@/layouts/components/BaseHeader";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalSidebarSink from "@/pages/pipelines/components/create/components/CreatePipelineModalSidebarSink";
import {
  CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH,
  CREATE_PIPELINE_MODAL_STEP_ORDER,
  CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_MAP,
  CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_VARIANT_MAP,
  CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_WEIGHT_MAP,
  CREATE_PIPELINE_MODAL_STEP_STATUS_TO_TEXT_VARIANT_MAP,
  CREATE_PIPELINE_MODAL_STEP_STATUS_TO_TEXT_WEIGHT_MAP,
  CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP,
} from "@/pages/pipelines/components/create/constants";
import {
  CreatePipelineModalStep,
  CreatePipelineModalStepStatus,
} from "@/pages/pipelines/components/create/types";

const SidebarWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  flex-shrink: 0;

  width: ${CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border-right: 0.5px solid ${({ theme }) => theme.color.border.primary};
`);

const StepButton = styled.button<{ $isClickable: boolean }>`
  display: flex;
  align-items: flex-start;
  text-align: left;
  gap: 8px;
  padding: 0;

  background-color: transparent;
  border: none;
  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "default")};
`;

const CreatePipelineModalSidebar = () => {
  const { stepIndex, sinks, activeSinkId, issuesBySink, isSubmitting } =
    useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const getStepStatus = (item: CreatePipelineModalStep): CreatePipelineModalStepStatus => {
    const itemIndex = CREATE_PIPELINE_MODAL_STEP_ORDER.indexOf(item);
    if (itemIndex < stepIndex) return CreatePipelineModalStepStatus.COMPLETED;
    if (itemIndex === stepIndex) return CreatePipelineModalStepStatus.CURRENT;
    return CreatePipelineModalStepStatus.UPCOMING;
  };

  return (
    <SidebarWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        <FlexWrapper direction={FlexDirection.COLUMN} padding="16px">
          <BaseHeader title="Create a new pipeline" />
        </FlexWrapper>
        <HorizontalDivider />
        <FlexWrapper direction={FlexDirection.COLUMN} padding="12px" fillWidth>
          <Widget variant={WidgetVariant.SECONDARY} fillWidth>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={12}>
              <Text
                size={TextSize.BODY_SM}
                weight={TextWeight.MEDIUM}
                variant={TextVariant.TERTIARY}
              >
                Steps
              </Text>
              {CREATE_PIPELINE_MODAL_STEP_ORDER.map((item) => {
                const status = getStepStatus(item);
                const isClickable =
                  status === CreatePipelineModalStepStatus.COMPLETED && !isSubmitting;
                const hasSinkRows =
                  item === CreatePipelineModalStep.RESOURCES &&
                  status !== CreatePipelineModalStepStatus.UPCOMING &&
                  sinks.length > 0;

                return (
                  <FlexWrapper key={item} direction={FlexDirection.COLUMN} gap={6} fillWidth>
                    <StepButton
                      $isClickable={isClickable}
                      onClick={
                        isClickable
                          ? () =>
                              dispatch({
                                type: CreatePipelineModalActionType.GO_TO_STEP,
                                payload: item,
                              })
                          : undefined
                      }
                    >
                      <Icon
                        component={CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_MAP[status]}
                        weight={CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_WEIGHT_MAP[status]}
                        variant={CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_VARIANT_MAP[status]}
                      />
                      <Text
                        variant={CREATE_PIPELINE_MODAL_STEP_STATUS_TO_TEXT_VARIANT_MAP[status]}
                        weight={CREATE_PIPELINE_MODAL_STEP_STATUS_TO_TEXT_WEIGHT_MAP[status]}
                      >
                        {CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP[item]}
                      </Text>
                    </StepButton>
                    {hasSinkRows && (
                      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
                        {sinks.map((sink) => (
                          <CreatePipelineModalSidebarSink
                            key={sink.connection.id}
                            sink={sink}
                            isActive={
                              sink.connection.id === activeSinkId &&
                              status === CreatePipelineModalStepStatus.CURRENT
                            }
                            issues={issuesBySink[sink.connection.id] ?? []}
                            onClick={
                              isSubmitting
                                ? undefined
                                : () =>
                                    dispatch({
                                      type: CreatePipelineModalActionType.OPEN_SINK_RESOURCES,
                                      payload: sink.connection.id,
                                    })
                            }
                          />
                        ))}
                      </FlexWrapper>
                    )}
                  </FlexWrapper>
                );
              })}
            </FlexWrapper>
          </Widget>
        </FlexWrapper>
      </FlexWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} padding="16px" fillWidth>
        <Paragraph weight={TextWeight.REGULAR} variant={TextVariant.TERTIARY}>
          Connect a source to one or more sinks, pick the resources you want to ingest, name your
          pipeline, and optionally set a schedule so it runs on its own.
        </Paragraph>
        <DocsButton
          label="Read the docs"
          path="/pipelines/create"
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.MEDIUM}
        />
      </FlexWrapper>
    </SidebarWrapper>
  );
};

export default CreatePipelineModalSidebar;
