import { useState } from "react";

import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";
import { match } from "ts-pattern";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import BaseHeader from "@/layouts/components/BaseHeader";

import { getNameError, isNameValid } from "@/pages/connectors/components/form/validation";
import { mapCanvasStateToVersionRequest } from "@/pages/pipelines/canvas/graph/serialize";
import CreatePipelineModalConnections from "@/pages/pipelines/components/create/CreatePipelineModalConnections";
import CreatePipelineModalDetails from "@/pages/pipelines/components/create/CreatePipelineModalDetails";
import CreatePipelineModalFooter from "@/pages/pipelines/components/create/CreatePipelineModalFooter";
import CreatePipelineModalSchedule from "@/pages/pipelines/components/create/CreatePipelineModalSchedule";
import CreatePipelineModalSidebar from "@/pages/pipelines/components/create/CreatePipelineModalSidebar";
import {
  CREATE_PIPELINE_MODAL_MAX_HEIGHT,
  CREATE_PIPELINE_MODAL_MIN_HEIGHT,
  CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP,
  CREATE_PIPELINE_MODAL_WIDTH,
} from "@/pages/pipelines/components/create/constants";
import {
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import {
  getDefaultPipelineName,
  mapCreatePipelineSelectionToCanvasState,
  mapCreatePipelineStateToRequest,
} from "@/pages/pipelines/components/create/utils";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useCreatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const ModalWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;

  min-height: ${CREATE_PIPELINE_MODAL_MIN_HEIGHT}px;
  max-height: ${CREATE_PIPELINE_MODAL_MAX_HEIGHT}px;
  width: ${CREATE_PIPELINE_MODAL_WIDTH}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const MainWrapper = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
`;

const BodyWrapper = withTheme(styled.div<PropsWithTheme<{ $isPadded: boolean }>>`
  display: flex;
  flex-direction: column;

  flex: 1;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
  background-color: ${({ theme }) => theme.color.background.base};
  padding: ${({ $isPadded }) => ($isPadded ? "16px" : "0")};
`);

interface CreatePipelineModalProps {
  onClose: () => void;
}

const DEFAULT_STATE: CreatePipelineModalState = {
  step: CreatePipelineModalStep.CONNECTIONS,
  sourceConnection: null,
  sinkConnections: [],
  name: "",
  isNameTouched: false,
  description: "",
  schedule: PIPELINE_SCHEDULE_DEFAULT_STATE,
  isSubmitting: false,
};

const CreatePipelineModal = ({ onClose }: CreatePipelineModalProps) => {
  const navigate = useNavigate();
  const { showToast } = useToast();

  const [state, setState] = useState<CreatePipelineModalState>(DEFAULT_STATE);

  const { mutate: createPipeline } = useCreatePipelineMutation();
  const { mutate: createPipelineVersion } = useCreatePipelineVersionMutation();

  const effectiveName = state.isNameTouched
    ? state.name
    : getDefaultPipelineName(state.sourceConnection, state.sinkConnections);

  const hasSelection = !!state.sourceConnection || state.sinkConnections.length > 0;
  const isScheduleValid =
    !state.schedule.isEnabled || formatPipelineScheduleSummary(state.schedule) !== null;

  const isNextDisabled = match(state.step)
    .with(CreatePipelineModalStep.CONNECTIONS, () => false)
    .with(CreatePipelineModalStep.DETAILS, () => !isNameValid(effectiveName))
    .with(CreatePipelineModalStep.SCHEDULE, () => !isNameValid(effectiveName) || !isScheduleValid)
    .exhaustive();

  const handleSourceSelect = (connection: Connection) => {
    setState((prev) => ({
      ...prev,
      sourceConnection: prev.sourceConnection?.id === connection.id ? null : connection,
    }));
  };

  const handleSinkToggle = (connection: Connection) => {
    setState((prev) => ({
      ...prev,
      sinkConnections: prev.sinkConnections.some((sink) => sink.id === connection.id)
        ? prev.sinkConnections.filter((sink) => sink.id !== connection.id)
        : [...prev.sinkConnections, connection],
    }));
  };

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name, isNameTouched: true }));
  };

  const handleDescriptionChange = (description: string) => {
    setState((prev) => ({ ...prev, description }));
  };

  const handleScheduleChange = (schedule: Partial<PipelineSettingsPageScheduleState>) => {
    setState((prev) => ({
      ...prev,
      schedule: { ...prev.schedule, ...schedule },
    }));
  };

  const handleBack = () => {
    const step = match(state.step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => CreatePipelineModalStep.CONNECTIONS)
      .with(CreatePipelineModalStep.DETAILS, () => CreatePipelineModalStep.CONNECTIONS)
      .with(CreatePipelineModalStep.SCHEDULE, () => CreatePipelineModalStep.DETAILS)
      .exhaustive();
    setState((prev) => ({ ...prev, step }));
  };

  const handleNext = () => {
    const step = match(state.step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => CreatePipelineModalStep.DETAILS)
      .with(CreatePipelineModalStep.DETAILS, () => CreatePipelineModalStep.SCHEDULE)
      .with(CreatePipelineModalStep.SCHEDULE, () => CreatePipelineModalStep.SCHEDULE)
      .exhaustive();
    setState((prev) => ({ ...prev, step }));
  };

  const handleStepClick = (step: CreatePipelineModalStep) => {
    setState((prev) => ({ ...prev, step }));
  };

  const handleNavigateToCanvas = (pipelineId: string) => {
    void navigate({ to: "/pipelines/$id/canvas", params: { id: pipelineId } });
  };

  const handleCreate = () => {
    setState((prev) => ({ ...prev, isSubmitting: true }));

    createPipeline(mapCreatePipelineStateToRequest(state, effectiveName), {
      onSuccess: (response) => {
        const pipelineId = response.pipeline?.id;
        if (!pipelineId) {
          setState((prev) => ({ ...prev, isSubmitting: false }));
          return;
        }

        if (!hasSelection) {
          showToast({
            variant: ToastVariant.SUCCESS,
            header: "Pipeline created",
            subheader: "Your pipeline has been created successfully.",
          });
          handleNavigateToCanvas(pipelineId);
          return;
        }

        const canvasState = mapCreatePipelineSelectionToCanvasState(
          state.sourceConnection,
          state.sinkConnections,
        );

        createPipelineVersion(mapCanvasStateToVersionRequest(canvasState, pipelineId, undefined), {
          onSuccess: () => {
            showToast({
              variant: ToastVariant.SUCCESS,
              header: "Pipeline created",
              subheader: "Your pipeline has been created successfully.",
            });
            handleNavigateToCanvas(pipelineId);
          },
          onError: (error) => {
            showToast({
              variant: ToastVariant.ERROR,
              header: "Pipeline created without connections",
              subheader: getErrorMessage(error, "Failed to add connections"),
            });
            handleNavigateToCanvas(pipelineId);
          },
        });
      },
      onError: (error) => {
        setState((prev) => ({ ...prev, isSubmitting: false }));
        showToast({
          variant: ToastVariant.ERROR,
          header: "Failed to create pipeline",
          subheader: getErrorMessage(error, "Failed to create pipeline"),
        });
      },
    });
  };

  const renderBody = () => {
    return match(state.step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => (
        <CreatePipelineModalConnections
          sourceConnection={state.sourceConnection}
          sinkConnections={state.sinkConnections}
          onSourceSelect={handleSourceSelect}
          onSinkToggle={handleSinkToggle}
        />
      ))
      .with(CreatePipelineModalStep.DETAILS, () => (
        <CreatePipelineModalDetails
          name={effectiveName}
          nameError={getNameError(effectiveName, state.isNameTouched) ?? undefined}
          description={state.description}
          onNameChange={handleNameChange}
          onDescriptionChange={handleDescriptionChange}
        />
      ))
      .with(CreatePipelineModalStep.SCHEDULE, () => (
        <CreatePipelineModalSchedule
          schedule={state.schedule}
          onScheduleChange={handleScheduleChange}
        />
      ))
      .exhaustive();
  };

  return (
    <ModalWrapper>
      <CreatePipelineModalSidebar
        step={state.step}
        isSubmitting={state.isSubmitting}
        onStepClick={handleStepClick}
      />
      <MainWrapper>
        <FlexItem grow={0} shrink={0}>
          <FlexWrapper padding="12px 16px" fillWidth>
            <BaseHeader
              title={CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP[state.step]}
              onClose={onClose}
            />
          </FlexWrapper>
        </FlexItem>
        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
        </FlexItem>

        <BodyWrapper $isPadded={state.step !== CreatePipelineModalStep.CONNECTIONS}>
          {renderBody()}
        </BodyWrapper>

        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
        </FlexItem>
        <CreatePipelineModalFooter
          step={state.step}
          hasSelection={hasSelection}
          isNextDisabled={isNextDisabled}
          isSubmitting={state.isSubmitting}
          onBack={handleBack}
          onNext={handleNext}
          onCreate={handleCreate}
        />
      </MainWrapper>
    </ModalWrapper>
  );
};

export default CreatePipelineModal;
