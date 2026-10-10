import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import {
  CreatePipelineScheduleRequestSchema,
  type PipelineScheduleConfig,
  UpdatePipelineScheduleRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/components/schedule/constants";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import type { PipelineScheduleState } from "@/pages/pipelines/components/schedule/types";
import {
  formatPipelineScheduleSummary,
  hasPipelineScheduleChanges,
} from "@/pages/pipelines/components/schedule/utils";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import {
  useCreatePipelineScheduleMutation,
  useUpdatePipelineScheduleMutation,
} from "@/api/queries/schedules";

import { getErrorMessage } from "@/utils/errors";

const PipelineSettingsPageSchedule: FC = () => {
  const { toast } = useToast();
  const { id: pipelineId } = usePipelineParams();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(pipelineId),
  });
  const schedule = data.pipeline?.schedule;

  const { mutate: createSchedule, isPending: isCreating } = useCreatePipelineScheduleMutation();
  const { mutate: updateSchedule, isPending: isUpdating } = useUpdatePipelineScheduleMutation();

  const createInitialState = (): PipelineScheduleState => ({
    ...PIPELINE_SCHEDULE_DEFAULT_STATE,
    cron: schedule?.config?.cron || PIPELINE_SCHEDULE_DEFAULT_STATE.cron,
    isEnabled: schedule?.config?.isEnabled ?? PIPELINE_SCHEDULE_DEFAULT_STATE.isEnabled,
    timezone: schedule?.config?.timezone || PIPELINE_SCHEDULE_DEFAULT_STATE.timezone,
  });

  const [state, setState] = useState<PipelineScheduleState>(createInitialState);

  const handleScheduleChange = (partial: Partial<PipelineScheduleState>) => {
    setState((prev) => ({ ...prev, ...partial }));
  };

  const handleCancel = () => {
    setState(createInitialState());
  };

  const handleSave = () => {
    const config: Omit<PipelineScheduleConfig, "$typeName" | "overlapPolicy"> = {
      cron: state.cron.trim(),
      timezone: state.timezone,
      isEnabled: state.isEnabled,
    };

    if (schedule?.config) {
      updateSchedule(
        create(UpdatePipelineScheduleRequestSchema, {
          pipelineId,
          schedule: { ...config, overlapPolicy: schedule.config.overlapPolicy },
        }),
        {
          onSuccess: () => {
            toast({
              header: "Schedule saved",
              description: "Your schedule has been saved successfully.",
              variant: ToastVariant.SUCCESS,
            });
          },
          onError: (error) => {
            toast({
              header: "Save failed",
              description: getErrorMessage(error, "Failed to save schedule"),
              variant: ToastVariant.ERROR,
            });
          },
        },
      );
      return;
    }

    createSchedule(
      create(CreatePipelineScheduleRequestSchema, {
        pipelineId,
        schedule: config,
      }),
      {
        onSuccess: () => {
          toast({
            header: "Schedule created",
            description: "Your pipeline will now run on a schedule.",
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          toast({
            header: "Create failed",
            description: getErrorMessage(error, "Failed to create schedule"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const summary = formatPipelineScheduleSummary(state);
  const hasChanges = hasPipelineScheduleChanges(state, schedule);
  const isDisabledDraft = !schedule && !state.isEnabled;
  const canSave = hasChanges && !isDisabledDraft && summary !== null;

  if (data.pipeline?.executionMode === ExecutionMode.CONTINUOUS) return null;

  return (
    <PipelineScheduleFields header="Schedule" state={state} onChange={handleScheduleChange}>
      <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
        <Button
          label="Cancel"
          variant={ButtonVariant.SECONDARY}
          isDisabled={!hasChanges || isCreating || isUpdating}
          onClick={handleCancel}
        />
        <Button
          label="Save"
          isDisabled={!canSave}
          isLoading={isCreating || isUpdating}
          onClick={handleSave}
        />
      </Flex>
    </PipelineScheduleFields>
  );
};

export default PipelineSettingsPageSchedule;
