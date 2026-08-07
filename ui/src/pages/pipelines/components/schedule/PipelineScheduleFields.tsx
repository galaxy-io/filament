import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import SwitcherInput from "@galaxy-io/dls/inputs/SwitcherInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
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

const PIPELINE_SCHEDULE_INPUT_WIDTH = 264;

interface PipelineScheduleFieldsProps {
  state: PipelineSettingsPageScheduleState;
  onChange: (partial: Partial<PipelineSettingsPageScheduleState>) => void;
}

const PipelineScheduleFields = ({ state, onChange }: PipelineScheduleFieldsProps) => {
  const handleEnabledChange = (enabled: boolean) => {
    onChange({ isEnabled: enabled });
  };

  const handleFrequencyChange = (frequency: PipelineScheduleFrequency) => {
    onChange({ frequency });
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
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
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
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
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
    </FlexWrapper>
  );
};

export default PipelineScheduleFields;
