import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import SwitcherInput from "@galaxy-io/dls/inputs/SwitcherInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS,
  PIPELINE_SCHEDULE_DAY_OPTIONS,
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
  isPipelineScheduleCronValid,
  mapPipelineScheduleStateToCron,
} from "@/pages/pipelines/settings/utils";

const PIPELINE_SCHEDULE_INPUT_WIDTH = 351;

const CollapsibleContent = styled.div<{ $isOpen: boolean }>`
  display: grid;
  grid-template-rows: ${({ $isOpen }) => ($isOpen ? "1fr" : "0fr")};
  transition: grid-template-rows 0.15s ease-in-out;
  width: 100%;
`;

const CollapsibleContentInner = styled.div`
  overflow: hidden;
`;

interface PipelineScheduleFieldsProps {
  state: PipelineSettingsPageScheduleState;
  onChange: (partial: Partial<PipelineSettingsPageScheduleState>) => void;
}

const PipelineScheduleFields = ({ state, onChange }: PipelineScheduleFieldsProps) => {
  const summary = formatPipelineScheduleSummary(state);
  const cronError =
    state.cron.trim() !== "" && !isPipelineScheduleCronValid(state.cron)
      ? "Use 5 fields: minute hour day month weekday"
      : undefined;

  const handleEnabledChange = (enabled: boolean) => {
    onChange({ isEnabled: enabled });
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

  const handleDaysChange = (options: SelectInputOption[]) => {
    onChange({ days: options.map((option) => option.value as number) });
  };

  const handleDayOfMonthChange = (option: SelectInputOption) => {
    onChange({ dayOfMonth: option.value as number });
  };

  const handleHourChange = (option: SelectInputOption) => {
    onChange({ hour: option.value as number });
  };

  const handleTimezoneChange = (option: SelectInputOption) => {
    onChange({ timezone: option.value as string });
  };

  const handleTimezoneSearch = (term: string, options: SelectInputOption[]) => {
    return options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()));
  };

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
    <Widget noPadding noHover fillWidth>
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          padding="16px"
          fillWidth
        >
          <Text variant={TextVariant.SECONDARY} weight={TextWeight.MEDIUM}>
            Enabled
          </Text>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
            {state.isEnabled && summary && (
              <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
                {summary}
              </Text>
            )}
            <ToggleInput
              size={InputSize.LARGE}
              value={state.isEnabled}
              onChange={handleEnabledChange}
            />
          </FlexWrapper>
        </FlexWrapper>
        <CollapsibleContent $isOpen={state.isEnabled}>
          <CollapsibleContentInner>
            <HorizontalDivider />
            <FlexWrapper
              direction={FlexDirection.COLUMN}
              gap={FlexGap.MEDIUM}
              padding="16px"
              fillWidth
            >
              <FlexWrapper
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.SPACE_BETWEEN}
                fillWidth
              >
                <Text variant={TextVariant.SECONDARY}>Frequency</Text>
                <SwitcherInput
                  items={frequencyItems}
                  size={InputSize.LARGE}
                  selectedId={state.frequency}
                />
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
              {state.frequency === PipelineScheduleFrequency.CUSTOM && (
                <FlexWrapper
                  alignItems={AlignItems.CENTER}
                  justifyContent={JustifyContent.SPACE_BETWEEN}
                  fillWidth
                >
                  <Text variant={TextVariant.SECONDARY}>Expression</Text>
                  <TextInput
                    value={state.cron}
                    onChange={handleCronChange}
                    error={cronError}
                    placeholder="0 * * * *"
                    size={InputSize.LARGE}
                    width={PIPELINE_SCHEDULE_INPUT_WIDTH}
                    isMonospace
                  />
                </FlexWrapper>
              )}
              {state.frequency !== PipelineScheduleFrequency.HOURLY &&
                state.frequency !== PipelineScheduleFrequency.CUSTOM && (
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
          </CollapsibleContentInner>
        </CollapsibleContent>
      </FlexWrapper>
    </Widget>
  );
};

export default PipelineScheduleFields;
