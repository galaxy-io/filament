import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { CalendarIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  CreatePipelineScheduleRequestSchema,
  GetPipelineRequestSchema,
  UpdatePipelineScheduleRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";
import {
  formatPipelineScheduleSummary,
  hasPipelineScheduleChanges,
  mapPipelineScheduleCronToState,
  mapPipelineScheduleStateToCron,
} from "@/pages/pipelines/settings/utils";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import {
  useCreatePipelineScheduleMutation,
  useUpdatePipelineScheduleMutation,
} from "@/api/queries/schedules";

import { getErrorMessage } from "@/utils/errors";

const PipelineSettingsPageSchedule = () => {
  const { showToast } = useToast();
  const { id: pipelineId } = useParams({ from: "/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id: pipelineId }),
  });
  const schedule = data.schedule;

  const { mutate: createSchedule, isPending: isCreating } = useCreatePipelineScheduleMutation();
  const { mutate: updateSchedule, isPending: isUpdating } = useUpdatePipelineScheduleMutation();

  const [state, setState] = useState<PipelineSettingsPageScheduleState>(() => ({
    ...PIPELINE_SCHEDULE_DEFAULT_STATE,
    ...mapPipelineScheduleCronToState(schedule?.config?.cron ?? ""),
    isEnabled: schedule?.config?.enabled ?? PIPELINE_SCHEDULE_DEFAULT_STATE.isEnabled,
    timezone: schedule?.config?.timezone || PIPELINE_SCHEDULE_DEFAULT_STATE.timezone,
  }));

  const handleScheduleChange = (partial: Partial<PipelineSettingsPageScheduleState>) => {
    setState((prev) => ({ ...prev, ...partial }));
  };

  const handleSave = () => {
    const config = {
      cron: mapPipelineScheduleStateToCron(state),
      timezone: state.timezone,
      enabled: state.isEnabled,
    };

    if (schedule?.config) {
      updateSchedule(
        create(UpdatePipelineScheduleRequestSchema, {
          pipelineId,
          schedule: { ...config, overlapPolicy: schedule.config.overlapPolicy },
        }),
        {
          onSuccess: () => {
            showToast({
              header: "Schedule saved",
              subheader: "Your schedule has been saved successfully.",
              variant: ToastVariant.SUCCESS,
            });
          },
          onError: (error) => {
            showToast({
              header: "Save failed",
              subheader: getErrorMessage(error, "Failed to save schedule"),
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
          showToast({
            header: "Schedule created",
            subheader: "Your pipeline will now run on a schedule.",
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          showToast({
            header: "Create failed",
            subheader: getErrorMessage(error, "Failed to create schedule"),
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

  return (
    <Accordion header="Schedule" icon={CalendarIcon} isOpenInitial>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
        <PipelineScheduleFields state={state} onChange={handleScheduleChange} />
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          fillWidth
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
            {state.isEnabled && summary}
          </Text>
          <Button
            label={"Save"}
            isDisabled={!canSave}
            isLoading={isCreating || isUpdating}
            onClick={handleSave}
          />
        </FlexWrapper>
      </FlexWrapper>
    </Accordion>
  );
};

export default PipelineSettingsPageSchedule;
