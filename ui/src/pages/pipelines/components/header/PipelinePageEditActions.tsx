import { type FC, useMemo } from "react";

import { ArrowUUpLeftIcon, FloppyDiskIcon, WarningIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Popover from "@galaxy-io/dls/overlays/Popover";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  mapCanvasStateToVersionRequest,
  mapPipelineVersionToCanvasState,
} from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasValidation } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  type PipelineCanvasValidationIssue,
  PipelineCanvasValidationIssueKind,
} from "@/pages/pipelines/canvas/types";
import PipelinePageSaveIssues from "@/pages/pipelines/components/header/PipelinePageSaveIssues";
import { usePipelineCanvasNavigate } from "@/pages/pipelines/hooks/usePipelineCanvasNavigate";

import { usePipelineParams } from "@/module/hooks";

import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";

const PipelinePageEditActions: FC = () => {
  const { toast } = useToast();
  const { id } = usePipelineParams();
  const state = usePipelineCanvasState();
  const { loadGraph } = usePipelineCanvasActions();
  const navigateCanvas = usePipelineCanvasNavigate();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const pipeline = data.pipeline;
  const currentVersion = pipeline?.currentVersion;

  const { mutate: createPipelineVersion, isPending: isSaving } = useCreatePipelineVersionMutation();
  const { issues, isPending: isValidating, isError } = usePipelineCanvasValidation();

  const saveIssues = useMemo<PipelineCanvasValidationIssue[]>(
    () => [
      ...issues,
      ...(isError
        ? [
            {
              kind: PipelineCanvasValidationIssueKind.GRAPH,
              message: "Unable to validate this pipeline.",
            },
          ]
        : []),
    ],
    [issues, isError],
  );

  if (!pipeline) {
    return null;
  }

  const handleUndo = () => {
    loadGraph(mapPipelineVersionToCanvasState(currentVersion));
  };

  const handleSelectResource = (edgeId: string) => {
    navigateCanvas({ node: undefined, resource: edgeId, showPanel: true });
  };

  const handleSave = () => {
    createPipelineVersion(
      mapCanvasStateToVersionRequest(state, id, currentVersion, pipeline.executionMode),
      {
        onSuccess: () => {
          toast({
            header: "Pipeline saved",
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          toast({
            header: "Save failed",
            description: getErrorMessage(error, "Failed to save pipeline"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  return (
    <>
      <Chip label="Unsaved changes" variant={ChipVariant.WARNING} hasDot />
      <Button
        label="Undo"
        icon={ArrowUUpLeftIcon}
        variant={ButtonVariant.SECONDARY}
        onClick={handleUndo}
      />
      {saveIssues.length > 0 && (
        <Popover
          placement={Placement.BOTTOM_END}
          ariaLabel="Save issues"
          body={
            <PipelinePageSaveIssues issues={saveIssues} onSelectResource={handleSelectResource} />
          }
        >
          <Button
            label={saveIssues.length.toString()}
            ariaLabel={pluralize("save issue", saveIssues.length, true)}
            leading={<Icon component={WarningIcon} variant={IconVariant.ERROR} />}
            variant={ButtonVariant.TERTIARY}
          />
        </Popover>
      )}
      <Button
        label="Save"
        icon={FloppyDiskIcon}
        variant={ButtonVariant.PRIMARY}
        isLoading={isSaving || isValidating}
        isDisabled={saveIssues.length > 0}
        onClick={handleSave}
      />
    </>
  );
};

export default PipelinePageEditActions;
