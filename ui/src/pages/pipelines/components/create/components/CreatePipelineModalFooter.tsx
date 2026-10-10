import type { FC } from "react";

import { ArrowLeftIcon, ArrowRightIcon, PlusIcon, WarningIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import BulletedList, { BulletedListSize } from "@galaxy-io/dls/lists/BulletedList";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  mapCreatePipelineNotifierToRequest,
  mapCreatePipelineStateToRequest,
  mapCreatePipelineStateToVersionRequest,
} from "@/pages/pipelines/components/create/serialize";
import type { PipelineNotifier } from "@/pages/pipelines/components/notifier/types";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useCreatePipelineNotifierMutation } from "@/api/queries/notifiers";
import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useCreatePipelineMutation } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const CreatePipelineModalFooter: FC = () => {
  const navigate = useFilamentNavigate();
  const { toast } = useToast();

  const state = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const { mutate: createPipeline } = useCreatePipelineMutation();
  const { mutate: createPipelineVersion } = useCreatePipelineVersionMutation();
  const { mutateAsync: createPipelineNotifier } = useCreatePipelineNotifierMutation();

  const { isBackVisible, isLastStep, isNextDisabled, isSubmitting, hints } = state;

  const handleNavigateToCanvas = (pipelineId: Pipeline["id"]) => {
    void navigate({ to: FilamentPath.PIPELINE_CANVAS, params: { id: pipelineId } });
  };

  const handleFinish = async (pipelineId: Pipeline["id"], notifiers: PipelineNotifier[]) => {
    for (const notifier of notifiers) {
      try {
        await createPipelineNotifier(mapCreatePipelineNotifierToRequest(notifier, pipelineId));
      } catch (error) {
        toast({
          variant: ToastVariant.ERROR,
          header: "Pipeline created without all notifiers",
          description: getErrorMessage(error, `Failed to add notifier ${notifier.name}`),
        });
        break;
      }
    }
    handleNavigateToCanvas(pipelineId);
  };

  const handleCreate = () => {
    dispatch({ type: CreatePipelineModalActionType.SET_SUBMITTING, payload: true });

    createPipeline(mapCreatePipelineStateToRequest(state, state.effectiveName), {
      onSuccess: (response) => {
        const pipelineId = response.pipeline?.id;
        if (!pipelineId) {
          dispatch({ type: CreatePipelineModalActionType.SET_SUBMITTING, payload: false });
          return;
        }

        const versionRequest = mapCreatePipelineStateToVersionRequest({
          sourceConnection: state.sourceConnection,
          rowsBySink: state.rowsBySink,
          sinks: state.sinks,
          nodeConfigs: state.nodeConfigs,
          replication: state.replication,
          executionMode: state.executionMode,
          pipelineId,
        });

        createPipelineVersion(versionRequest, {
          onSuccess: () => {
            toast({
              variant: ToastVariant.SUCCESS,
              header: "Pipeline created",
              description: "Your pipeline has been created successfully.",
            });
            void handleFinish(pipelineId, state.notifiers);
          },
          onError: (error) => {
            toast({
              variant: ToastVariant.ERROR,
              header: "Pipeline created without connections",
              description: getErrorMessage(error, "Failed to add connections"),
            });
            void handleFinish(pipelineId, state.notifiers);
          },
        });
      },
      onError: (error) => {
        dispatch({ type: CreatePipelineModalActionType.SET_SUBMITTING, payload: false });
        toast({
          variant: ToastVariant.ERROR,
          header: "Failed to create pipeline",
          description: getErrorMessage(error, "Failed to create pipeline"),
        });
      },
    });
  };

  const renderAction = () => {
    if (isLastStep) {
      return (
        <Button
          label={isSubmitting ? "Creating..." : "Create pipeline"}
          icon={PlusIcon}
          isLoading={isSubmitting}
          isDisabled={isSubmitting || isNextDisabled}
          onClick={handleCreate}
        />
      );
    }

    return (
      <Button
        label="Next"
        icon={ArrowRightIcon}
        onClick={() => dispatch({ type: CreatePipelineModalActionType.GO_NEXT })}
        isDisabled={isNextDisabled}
        isIconTrailing
      />
    );
  };

  return (
    <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.SPACE_BETWEEN} fillWidth>
      <Button
        label="Back"
        icon={ArrowLeftIcon}
        variant={ButtonVariant.SECONDARY}
        onClick={() => dispatch({ type: CreatePipelineModalActionType.GO_BACK })}
        isDisabled={isSubmitting || !isBackVisible}
      />
      <Flex alignItems={AlignItems.CENTER} gap={12} grow={0} shrink={0}>
        {hints.length > 0 && (
          <Chip
            label="Invalid"
            icon={WarningIcon}
            variant={ChipVariant.ERROR}
            size={ChipSize.LARGE}
            tooltip={<BulletedList items={hints} size={BulletedListSize.SMALL} />}
          />
        )}
        {renderAction()}
      </Flex>
    </Flex>
  );
};

export default CreatePipelineModalFooter;
