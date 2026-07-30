import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { CalendarIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  type SelectInputOption,
  SelectInputSize,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  CreatePipelineScheduleRequestSchema,
  type PipelineSchedule,
  UpdatePipelineScheduleRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  getPipelineScheduleCronError,
  hasPipelineScheduleChanges,
  mapPipelineScheduleCronToCanonical,
  PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
} from "@/pages/pipelines/settings/utils";

import {
  useCreatePipelineScheduleMutation,
  useUpdatePipelineScheduleMutation,
} from "@/api/queries/schedules";

import { getErrorMessage } from "@/utils/errors";

const PIPELINE_SCHEDULE_INPUT_WIDTH = 300;

export const PIPELINE_SCHEDULE_TIMEZONE_OPTIONS: SelectInputOption[] = Intl.supportedValuesOf(
  "timeZone",
).map((timezone) => ({
  id: timezone,
  label: timezone,
  value: timezone,
}));

interface PipelineSettingsPageScheduleProps {
  pipelineId: string;
  schedule?: PipelineSchedule;
}

export interface PipelineSettingsPageScheduleState {
  enabled: boolean;
  cron: string;
  timezone: string;
}

const DEFAULT_STATE: PipelineSettingsPageScheduleState = {
  enabled: false,
  cron: "",
  timezone: PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
};

const PipelineSettingsPageSchedule = ({
  pipelineId,
  schedule,
}: PipelineSettingsPageScheduleProps) => {
  const { showToast } = useToast();

  const { mutate: createSchedule, isPending: isCreating } = useCreatePipelineScheduleMutation();
  const { mutate: updateSchedule, isPending: isUpdating } = useUpdatePipelineScheduleMutation();

  const [state, setState] = useState<PipelineSettingsPageScheduleState>(() => ({
    ...DEFAULT_STATE,
    enabled: schedule?.config?.enabled ?? DEFAULT_STATE.enabled,
    cron: schedule?.config?.cron ?? DEFAULT_STATE.cron,
    timezone: schedule?.config?.timezone || DEFAULT_STATE.timezone,
  }));

  const handleEnabledChange = (enabled: boolean) => {
    setState((prev) => ({ ...prev, enabled }));
  };

  const handleCronChange = (cron: string) => {
    setState((prev) => ({ ...prev, cron }));
  };

  const handleTimezoneChange = (option: SelectInputOption) => {
    setState((prev) => ({ ...prev, timezone: option.value as string }));
  };

  const handleTimezoneSearch = (term: string, options: SelectInputOption[]) => {
    return options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()));
  };

  const handleSave = () => {
    const config = {
      cron: mapPipelineScheduleCronToCanonical(state.cron),
      timezone: state.timezone,
      enabled: state.enabled,
    };

    if (schedule?.config) {
      updateSchedule(
        create(UpdatePipelineScheduleRequestSchema, {
          pipelineId,
          schedule: { ...config, overlapPolicy: schedule.config.overlapPolicy },
        }),
        {
          onSuccess: () => {
            setState((prev) => ({ ...prev, cron: config.cron }));
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
          setState((prev) => ({ ...prev, cron: config.cron }));
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

  const cronError = getPipelineScheduleCronError(state.cron);
  const hasChanges = hasPipelineScheduleChanges(state, schedule);
  const isDisabledDraft = !schedule && !state.enabled;
  const canSave = hasChanges && cronError === null && !isDisabledDraft;

  const selectedTimezoneOption =
    PIPELINE_SCHEDULE_TIMEZONE_OPTIONS.find((option) => option.value === state.timezone) ?? null;

  return (
    <Accordion header="Schedule" icon={CalendarIcon} isOpenInitial>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
        <Widget noHover fillWidth>
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY}>Enabled</Text>
            <ToggleInput value={state.enabled} onChange={handleEnabledChange} />
          </FlexWrapper>
        </Widget>
        <Widget noHover fillWidth>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              fillWidth
            >
              <Text variant={TextVariant.SECONDARY}>Cron expression</Text>
              <TextInput
                value={state.cron}
                onChange={handleCronChange}
                size={InputSize.LARGE}
                placeholder="0 0 * * *"
                width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                error={state.cron.trim() ? (cronError ?? undefined) : undefined}
                isDisabled={!state.enabled}
                isMonospace
              />
            </FlexWrapper>
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              fillWidth
            >
              <Text variant={TextVariant.SECONDARY}>Timezone</Text>
              <SelectInput
                options={PIPELINE_SCHEDULE_TIMEZONE_OPTIONS}
                value={selectedTimezoneOption}
                onChange={handleTimezoneChange}
                onSearch={handleTimezoneSearch}
                debounceMs={100}
                size={SelectInputSize.LARGE}
                width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                isDisabled={!state.enabled}
              />
            </FlexWrapper>
          </FlexWrapper>
        </Widget>
        <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
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
