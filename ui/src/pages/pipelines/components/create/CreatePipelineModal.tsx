import { useCallback, useState } from "react";

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

import type { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import BaseHeader from "@/layouts/components/BaseHeader";

import { getNameError, isNameValid } from "@/pages/connectors/components/form/validation";
import CreatePipelineModalConnections from "@/pages/pipelines/components/create/CreatePipelineModalConnections";
import CreatePipelineModalDelivery from "@/pages/pipelines/components/create/CreatePipelineModalDelivery";
import CreatePipelineModalDetails from "@/pages/pipelines/components/create/CreatePipelineModalDetails";
import CreatePipelineModalFooter from "@/pages/pipelines/components/create/CreatePipelineModalFooter";
import CreatePipelineModalResources from "@/pages/pipelines/components/create/CreatePipelineModalResources";
import CreatePipelineModalSidebar from "@/pages/pipelines/components/create/CreatePipelineModalSidebar";
import {
  CREATE_PIPELINE_MODAL_HEIGHT,
  CREATE_PIPELINE_MODAL_STEP_ORDER,
  CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP,
  CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP,
  CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP,
  CREATE_PIPELINE_MODAL_WIDTH,
} from "@/pages/pipelines/components/create/constants";
import { useCreatePipelineResources } from "@/pages/pipelines/components/create/hooks/useCreatePipelineResources";
import {
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import {
  getDefaultPipelineName,
  mapCreatePipelineStateToRequest,
  mapCreatePipelineStateToVersionRequest,
} from "@/pages/pipelines/components/create/utils";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useCreatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const ModalWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;

  height: ${CREATE_PIPELINE_MODAL_HEIGHT}px;
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
  activeSinkId: "",
  sourceConnection: null,
  sinkConnections: [],
  resourceSelection: {},
  resourceReadModes: {},
  resourceCursors: {},
  sinkWriteModes: {},
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

  const {
    rowsBySink,
    sinks,
    replication,
    isCdc,
    blockingMessages,
    blockingSinkIds,
    selectedCountBySink,
    hasEmptySink,
    isLoading,
    discoverError,
  } = useCreatePipelineResources(state);

  const effectiveName = state.isNameTouched
    ? state.name
    : getDefaultPipelineName(state.sourceConnection, state.sinkConnections);

  const isScheduleValid =
    !state.schedule.isEnabled || formatPipelineScheduleSummary(state.schedule) !== null;
  const isConnectionsValid = !!state.sourceConnection && state.sinkConnections.length > 0;
  const isResourcesValid = (!!discoverError || !hasEmptySink) && !blockingMessages.length;

  const isNextDisabled = match(state.step)
    .with(CreatePipelineModalStep.CONNECTIONS, () => !isConnectionsValid)
    .with(CreatePipelineModalStep.RESOURCES, () => !isResourcesValid)
    .with(CreatePipelineModalStep.DELIVERY, () => !isScheduleValid)
    .with(CreatePipelineModalStep.DETAILS, () => !isNameValid(effectiveName))
    .exhaustive();

  const activeSinkId = sinks.some((sink) => sink.connection.id === state.activeSinkId)
    ? state.activeSinkId
    : (sinks[0]?.connection.id ?? "");

  const stepIndex = CREATE_PIPELINE_MODAL_STEP_ORDER.indexOf(state.step);
  const hint = blockingMessages.length
    ? blockingMessages.join("\n")
    : CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP[state.step];

  const handleSourceSelect = (connection: Connection) => {
    setState((prev) => ({
      ...prev,
      sourceConnection: prev.sourceConnection?.id === connection.id ? null : connection,
      resourceSelection: {},
      resourceReadModes: {},
      resourceCursors: {},
    }));
  };

  const handleResourceSelectionChange = useCallback(
    (sinkId: string, visibleNames: string[], selection: Record<string, boolean>) => {
      setState((prev) => ({
        ...prev,
        resourceSelection: {
          ...prev.resourceSelection,
          [sinkId]: {
            ...prev.resourceSelection[sinkId],
            ...Object.fromEntries(visibleNames.map((name) => [name, !!selection[name]])),
          },
        },
      }));
    },
    [],
  );

  const handleResourceReadModeChange = useCallback(
    (sinkId: string, resource: string, readMode: ReadMode) => {
      setState((prev) => ({
        ...prev,
        resourceReadModes: {
          ...prev.resourceReadModes,
          [sinkId]: { ...prev.resourceReadModes[sinkId], [resource]: readMode },
        },
      }));
    },
    [],
  );

  const handleResourceCursorChange = useCallback(
    (sinkId: string, resource: string, cursorField: string) => {
      setState((prev) => ({
        ...prev,
        resourceCursors: {
          ...prev.resourceCursors,
          [sinkId]: { ...prev.resourceCursors[sinkId], [resource]: cursorField },
        },
      }));
    },
    [],
  );

  const handleSinkWriteModeChange = (sinkId: string, writeMode: WriteMode) => {
    setState((prev) => ({
      ...prev,
      sinkWriteModes: { ...prev.sinkWriteModes, [sinkId]: writeMode },
    }));
  };

  const handleSinkSelect = (sinkId: string) => {
    setState((prev) => ({ ...prev, activeSinkId: sinkId }));
  };

  const handleSinkClick = (sinkId: string) => {
    setState((prev) => ({
      ...prev,
      activeSinkId: sinkId,
      step: CreatePipelineModalStep.RESOURCES,
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
    setState((prev) => ({
      ...prev,
      step: CREATE_PIPELINE_MODAL_STEP_ORDER[stepIndex - 1] ?? prev.step,
    }));
  };

  const handleNext = () => {
    setState((prev) => ({
      ...prev,
      step: CREATE_PIPELINE_MODAL_STEP_ORDER[stepIndex + 1] ?? prev.step,
    }));
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

        const versionRequest = mapCreatePipelineStateToVersionRequest({
          sourceConnection: state.sourceConnection,
          rowsBySink,
          sinks,
          replication,
          pipelineId,
        });

        createPipelineVersion(versionRequest, {
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
      .with(CreatePipelineModalStep.RESOURCES, () => (
        <CreatePipelineModalResources
          rows={rowsBySink[activeSinkId] ?? []}
          sinks={sinks}
          activeSinkId={activeSinkId}
          selectedCountBySink={selectedCountBySink}
          blockingSinkIds={blockingSinkIds}
          isCdc={isCdc}
          isLoading={isLoading}
          discoverError={discoverError}
          onSinkSelect={handleSinkSelect}
          onSelectionChange={handleResourceSelectionChange}
          onReadModeChange={handleResourceReadModeChange}
          onCursorChange={handleResourceCursorChange}
        />
      ))
      .with(CreatePipelineModalStep.DELIVERY, () => (
        <CreatePipelineModalDelivery
          sinks={sinks}
          isCdc={isCdc}
          schedule={state.schedule}
          onSinkWriteModeChange={handleSinkWriteModeChange}
          onScheduleChange={handleScheduleChange}
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
      .exhaustive();
  };

  return (
    <ModalWrapper>
      <CreatePipelineModalSidebar
        step={state.step}
        sinks={sinks}
        activeSinkId={activeSinkId}
        selectedCountBySink={selectedCountBySink}
        blockingSinkIds={blockingSinkIds}
        isSubmitting={state.isSubmitting}
        onStepClick={handleStepClick}
        onSinkClick={handleSinkClick}
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

        <BodyWrapper $isPadded={CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP[state.step]}>
          {renderBody()}
        </BodyWrapper>

        <FlexItem grow={0} shrink={0}>
          <HorizontalDivider />
        </FlexItem>
        <CreatePipelineModalFooter
          isBackVisible={stepIndex > 0}
          isLastStep={stepIndex === CREATE_PIPELINE_MODAL_STEP_ORDER.length - 1}
          isNextDisabled={isNextDisabled}
          isSubmitting={state.isSubmitting}
          hint={isNextDisabled ? hint : undefined}
          onBack={handleBack}
          onNext={handleNext}
          onCreate={handleCreate}
        />
      </MainWrapper>
    </ModalWrapper>
  );
};

export default CreatePipelineModal;
