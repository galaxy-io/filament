import { type ComponentProps, type PropsWithChildren, useState } from "react";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput, { MultiSelectInputSize } from "@galaxy-io/dls/inputs/MultiSelectInput";
import SelectInput, { SelectInputSize } from "@galaxy-io/dls/inputs/SelectInput";
import SwitchInput, { SwitchInputSize } from "@galaxy-io/dls/inputs/SwitchInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ToggleInput, { ToggleInputSize, type ToggleOption } from "@galaxy-io/dls/inputs/ToggleInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS,
  PIPELINE_SCHEDULE_DAY_OPTIONS,
  PIPELINE_SCHEDULE_FREQUENCY_OPTIONS,
  PIPELINE_SCHEDULE_HOUR_OPTIONS,
  PIPELINE_SCHEDULE_TIMEZONE_OPTIONS,
  PIPELINE_SETTINGS_INPUT_WIDTH,
} from "@/pages/pipelines/settings/constants";
import {
  PipelineScheduleFrequency,
  type PipelineSettingsPageScheduleState,
} from "@/pages/pipelines/settings/types";
import {
  formatPipelineScheduleSummary,
  isPipelineScheduleCronValid,
  mapPipelineScheduleStateToCron,
} from "@/pages/pipelines/settings/utils";

interface PipelineScheduleFieldsProps {
  header: string;
  size?: ComponentProps<typeof Widget>["size"];
  isOpenInitial?: boolean;
  state: PipelineSettingsPageScheduleState;
  onChange: (partial: Partial<PipelineSettingsPageScheduleState>) => void;
}

const PipelineScheduleFields = ({
  header,
  size,
  isOpenInitial = false,
  state,
  onChange,
  children,
}: PropsWithChildren<PipelineScheduleFieldsProps>) => {
  const [isOpen, setIsOpen] = useState(isOpenInitial);

  const summary = formatPipelineScheduleSummary(state);
  const cronError =
    state.cron.trim() !== "" && !isPipelineScheduleCronValid(state.cron)
      ? "Use 5 fields: minute hour day month weekday"
      : undefined;

  const handleEnabledChange = (isEnabled: boolean) => {
    setIsOpen(true);
    onChange({ isEnabled });
  };

  const handleFrequencyChange = (frequency: PipelineScheduleFrequency) => {
    if (frequency === PipelineScheduleFrequency.CUSTOM) {
      onChange({ frequency, cron: state.cron || mapPipelineScheduleStateToCron(state) });
      return;
    }
    onChange({ frequency });
  };

  const handleCronChange = (cron: string) => {
    onChange({ cron });
  };

  const handleDaysChange = (ids: string[]) => {
    onChange({ days: ids.map(Number) });
  };

  const handleDayOfMonthChange = (id: string | null) => {
    if (id !== null) onChange({ dayOfMonth: Number(id) });
  };

  const handleHourChange = (id: string | null) => {
    if (id !== null) onChange({ hour: Number(id) });
  };

  const handleTimezoneChange = (id: string | null) => {
    if (id !== null) onChange({ timezone: id });
  };

  const frequencyItems: ToggleOption<PipelineScheduleFrequency>[] =
    PIPELINE_SCHEDULE_FREQUENCY_OPTIONS.map((option) => ({
      id: option.frequency,
      label: option.label,
    }));

  return (
    <Widget
      isCollapsible
      header={header}
      size={size}
      isOpen={isOpen}
      onOpenChange={() => setIsOpen((prev) => !prev)}
      actions={
        <Flex alignItems={AlignItems.CENTER} gap={12}>
          {state.isEnabled && summary && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
              {summary}
            </Text>
          )}
          <SwitchInput
            size={SwitchInputSize.MEDIUM}
            isChecked={state.isEnabled}
            onChange={handleEnabledChange}
          />
        </Flex>
      }
    >
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
        <Flex
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          fillWidth
        >
          <Text variant={TextVariant.SECONDARY}>Frequency</Text>
          <ToggleInput
            options={frequencyItems}
            size={ToggleInputSize.LARGE}
            value={state.frequency}
            onChange={handleFrequencyChange}
          />
        </Flex>
        {state.frequency === PipelineScheduleFrequency.WEEKLY && (
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY}>Run on</Text>
            <Box width={PIPELINE_SETTINGS_INPUT_WIDTH}>
              <MultiSelectInput
                fillWidth
                options={PIPELINE_SCHEDULE_DAY_OPTIONS}
                value={state.days.map(String)}
                onChange={handleDaysChange}
                size={MultiSelectInputSize.LARGE}
                placeholder="Select days"
              />
            </Box>
          </Flex>
        )}
        {state.frequency === PipelineScheduleFrequency.MONTHLY && (
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY}>On the</Text>
            <Box width={PIPELINE_SETTINGS_INPUT_WIDTH}>
              <SelectInput
                fillWidth
                options={PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS}
                value={String(state.dayOfMonth)}
                onChange={handleDayOfMonthChange}
                size={SelectInputSize.LARGE}
              />
            </Box>
          </Flex>
        )}
        {state.frequency === PipelineScheduleFrequency.CUSTOM && (
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY}>Expression</Text>
            <Box width={PIPELINE_SETTINGS_INPUT_WIDTH}>
              <TextInput
                fillWidth
                value={state.cron}
                onChange={handleCronChange}
                error={cronError}
                placeholder="0 * * * *"
                size={InputSize.LARGE}
                family={FontFamily.MONO}
              />
            </Box>
          </Flex>
        )}
        {state.frequency !== PipelineScheduleFrequency.HOURLY &&
          state.frequency !== PipelineScheduleFrequency.CUSTOM && (
            <Flex
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              fillWidth
            >
              <Text variant={TextVariant.SECONDARY}>At</Text>
              <Box width={PIPELINE_SETTINGS_INPUT_WIDTH}>
                <SelectInput
                  fillWidth
                  options={PIPELINE_SCHEDULE_HOUR_OPTIONS}
                  value={String(state.hour)}
                  onChange={handleHourChange}
                  size={SelectInputSize.LARGE}
                />
              </Box>
            </Flex>
          )}
        {state.frequency !== PipelineScheduleFrequency.HOURLY && (
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            fillWidth
          >
            <Text variant={TextVariant.SECONDARY}>Timezone</Text>
            <Box width={PIPELINE_SETTINGS_INPUT_WIDTH}>
              <SelectInput
                fillWidth
                options={PIPELINE_SCHEDULE_TIMEZONE_OPTIONS}
                value={state.timezone}
                onChange={handleTimezoneChange}
                isSearchable
                size={SelectInputSize.LARGE}
              />
            </Box>
          </Flex>
        )}
        {children}
      </Flex>
    </Widget>
  );
};

export default PipelineScheduleFields;
