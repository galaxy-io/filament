import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, PlusIcon, WarningIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import BulletedList, { BulletedListSize } from "@galaxy-io/dls/lists/BulletedList";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  mapCreatePipelineStateToRequest,
  mapCreatePipelineStateToVersionRequest,
} from "@/pages/pipelines/components/create/serialize";

import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useCreatePipelineMutation } from "@/api/queries/pipelines";

import { NOOP } from "@/constants";

import { getErrorMessage } from "@/utils/errors";

const FooterWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const CreatePipelineModalFooter = () => {
  const navigate = useNavigate();
  const { showToast } = useToast();

  const state = useCreatePipelineModalState();
  const { goBack, goNext, setSubmitting } = useCreatePipelineModalActions();

  const { mutate: createPipeline } = useCreatePipelineMutation();
  const { mutate: createPipelineVersion } = useCreatePipelineVersionMutation();

  const { isBackVisible, isLastStep, isNextDisabled, isSubmitting, hints } = state;

  const handleNavigateToCanvas = (pipelineId: string) => {
    void navigate({ to: "/pipelines/$id/canvas", params: { id: pipelineId } });
  };

  const handleCreate = () => {
    setSubmitting(true);

    createPipeline(mapCreatePipelineStateToRequest(state, state.effectiveName), {
      onSuccess: (response) => {
        const pipelineId = response.pipeline?.id;
        if (!pipelineId) {
          setSubmitting(false);
          return;
        }

        const versionRequest = mapCreatePipelineStateToVersionRequest({
          sourceConnection: state.sourceConnection,
          rowsBySink: state.rowsBySink,
          sinks: state.sinks,
          replication: state.replication,
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
        setSubmitting(false);
        showToast({
          variant: ToastVariant.ERROR,
          header: "Failed to create pipeline",
          subheader: getErrorMessage(error, "Failed to create pipeline"),
        });
      },
    });
  };

  const renderAction = () => {
    if (isSubmitting) {
      return (
        <Button size={ButtonSize.LARGE} label="Creating..." onClick={NOOP} isLoading isDisabled />
      );
    }

    if (isLastStep) {
      return (
        <Button
          size={ButtonSize.LARGE}
          label="Create pipeline"
          icon={PlusIcon}
          onClick={handleCreate}
          isDisabled={isNextDisabled}
        />
      );
    }

    return (
      <Button
        size={ButtonSize.LARGE}
        label="Next"
        icon={ArrowRightIcon}
        onClick={goNext}
        isDisabled={isNextDisabled}
        isIconTrailing
      />
    );
  };

  return (
    <FooterWrapper>
      <Button
        size={ButtonSize.LARGE}
        label="Back"
        icon={ArrowLeftIcon}
        variant={ButtonVariant.SECONDARY}
        onClick={goBack}
        isDisabled={isSubmitting || !isBackVisible}
      />
      <FlexWrapper alignItems={AlignItems.CENTER} gap={12} grow={0} shrink={0}>
        {hints.length > 0 && (
          <Tooltip
            body={<BulletedList items={hints} size={BulletedListSize.SMALL} />}
            position={TooltipPosition.TOP}
          >
            <Chip
              label="Invalid"
              icon={WarningIcon}
              variant={ChipVariant.ERROR}
              size={ChipSize.LARGE}
            />
          </Tooltip>
        )}
        {renderAction()}
      </FlexWrapper>
    </FooterWrapper>
  );
};

export default CreatePipelineModalFooter;
