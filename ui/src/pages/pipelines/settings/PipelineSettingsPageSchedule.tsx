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
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Switcher from "@galaxy-io/dls/switcher/Switcher";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  CreatePipelineScheduleRequestSchema,
  type PipelineSchedule,
  UpdatePipelineScheduleRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS,
  PIPELINE_SCHEDULE_DAY_OPTIONS,
  PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
  PIPELINE_SCHEDULE_FREQUENCY_OPTIONS,
  PIPELINE_SCHEDULE_HOUR_OPTIONS,
  PIPELINE_SCHEDULE_TIMEZONE_OPTIONS,
} from "@/pages/pipelines/settings/constants";
import {
  PipelineScheduleFrequency,
  type PipelineSettingsPageScheduleState,
} from "@/pages/pipelines/settings/types";
import {
  formatPipelineScheduleSummary,
  hasPipelineScheduleChanges,
  mapPipelineScheduleCronToState,
  mapPipelineScheduleStateToCron,
} from "@/pages/pipelines/settings/utils";

import {
  useCreatePipelineScheduleMutation,
  useUpdatePipelineScheduleMutation,
} from "@/api/queries/schedules";

import { getErrorMessage } from "@/utils/errors";

const PIPELINE_SCHEDULE_INPUT_WIDTH = 300;

interface PipelineSettingsPageScheduleProps {
  pipelineId: string;
  schedule?: PipelineSchedule;
}

const DEFAULT_STATE: PipelineSettingsPageScheduleState = {
  isEnabled: false,
  frequency: PipelineScheduleFrequency.DAILY,
  days: [1],
  dayOfMonth: 1,
  hour: 9,
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
    ...mapPipelineScheduleCronToState(schedule?.config?.cron ?? ""),
    isEnabled: schedule?.config?.enabled ?? DEFAULT_STATE.isEnabled,
    timezone: schedule?.config?.timezone || DEFAULT_STATE.timezone,
  }));

  const handleEnabledChange = (enabled: boolean) => {
    setState((prev) => ({ ...prev, isEnabled: enabled }));
  };

  const handleFrequencyChange = (frequency: PipelineScheduleFrequency) => {
    setState((prev) => ({ ...prev, frequency }));
  };

  const handleDaysChange = (options: SelectInputOption[]) => {
    setState((prev) => ({
      ...prev,
      days: options.map((option) => option.value as number),
    }));
  };

  const handleDayOfMonthChange = (option: SelectInputOption) => {
    setState((prev) => ({ ...prev, dayOfMonth: option.value as number }));
  };

  const handleHourChange = (option: SelectInputOption) => {
    setState((prev) => ({ ...prev, hour: option.value as number }));
  };

  const handleTimezoneChange = (option: SelectInputOption) => {
    setState((prev) => ({ ...prev, timezone: option.value as string }));
  };

  const handleTimezoneSearch = (term: string, options: SelectInputOption[]) => {
    return options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()));
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

  const frequencyItems = PIPELINE_SCHEDULE_FREQUENCY_OPTIONS.map((option) => ({
    id: option.frequency,
    label: option.label,
    onClick: () => handleFrequencyChange(option.frequency),
  }));

  const selectedDayOptions = PIPELINE_SCHEDULE_DAY_OPTIONS.filter((option) =>
    state.days.includes(option.value as number),
  );
  const selectedHourOption =
    PIPELINE_SCHEDULE_HOUR_OPTIONS.find((option) => option.value === state.hour) ?? null;
  const selectedDayOfMonthOption =
    PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS.find((option) => option.value === state.dayOfMonth) ??
    null;
  const selectedTimezoneOption =
    PIPELINE_SCHEDULE_TIMEZONE_OPTIONS.find((option) => option.value === state.timezone) ?? null;

  return (
    <Accordion header="Schedule" icon={CalendarIcon} isOpenInitial>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
        <Widget noHover fillWidth>
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY}>Enabled</Text>
            <ToggleInput value={state.isEnabled} onChange={handleEnabledChange} />
          </FlexWrapper>
        </Widget>
        <Widget noHover fillWidth>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              fillWidth
            >
              <Text variant={TextVariant.SECONDARY}>Frequency</Text>
              <Switcher items={frequencyItems} selectedId={state.frequency} />
            </FlexWrapper>
            {state.frequency === PipelineScheduleFrequency.WEEKLY && (
              <FlexWrapper
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.SPACE_BETWEEN}
                fillWidth
              >
                <Text variant={TextVariant.SECONDARY}>Run on</Text>
                <MultiSelectInput
                  options={PIPELINE_SCHEDULE_DAY_OPTIONS}
                  value={selectedDayOptions}
                  onChange={handleDaysChange}
                  size={InputSize.LARGE}
                  width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                  placeholder="Select days"
                />
              </FlexWrapper>
            )}
            {state.frequency === PipelineScheduleFrequency.MONTHLY && (
              <FlexWrapper
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.SPACE_BETWEEN}
                fillWidth
              >
                <Text variant={TextVariant.SECONDARY}>On the</Text>
                <SelectInput
                  options={PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS}
                  value={selectedDayOfMonthOption}
                  onChange={handleDayOfMonthChange}
                  size={InputSize.LARGE}
                  width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                />
              </FlexWrapper>
            )}
            {state.frequency !== PipelineScheduleFrequency.HOURLY && (
              <FlexWrapper
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.SPACE_BETWEEN}
                fillWidth
              >
                <Text variant={TextVariant.SECONDARY}>At</Text>
                <SelectInput
                  options={PIPELINE_SCHEDULE_HOUR_OPTIONS}
                  value={selectedHourOption}
                  onChange={handleHourChange}
                  size={InputSize.LARGE}
                  width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                />
              </FlexWrapper>
            )}
            {state.frequency !== PipelineScheduleFrequency.HOURLY && (
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
                  size={InputSize.LARGE}
                  width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                />
              </FlexWrapper>
            )}
          </FlexWrapper>
        </Widget>
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          fillWidth
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.PRIMARY}>
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
