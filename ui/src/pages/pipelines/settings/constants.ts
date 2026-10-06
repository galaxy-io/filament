import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  PipelineScheduleFrequency,
  type PipelineScheduleFrequencyOption,
  type PipelineSettingsPageScheduleState,
} from "@/pages/pipelines/settings/types";

export const PIPELINE_SETTINGS_INPUT_WIDTH = 351;

export const PIPELINE_SCHEDULE_DEFAULT_TIMEZONE = Intl.DateTimeFormat().resolvedOptions().timeZone;

export const PIPELINE_SCHEDULE_DEFAULT_STATE: PipelineSettingsPageScheduleState = {
  isEnabled: false,
  frequency: PipelineScheduleFrequency.DAILY,
  days: [1],
  dayOfMonth: 1,
  hour: 9,
  cron: "",
  timezone: PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
};

export const PIPELINE_SCHEDULE_TIMEZONE_OPTIONS: SelectOption[] = Intl.supportedValuesOf(
  "timeZone",
).map((timezone) => ({
  id: timezone,
  label: timezone,
}));

export const PIPELINE_SCHEDULE_FREQUENCY_OPTIONS: PipelineScheduleFrequencyOption[] = [
  { frequency: PipelineScheduleFrequency.HOURLY, label: "Hourly" },
  { frequency: PipelineScheduleFrequency.DAILY, label: "Daily" },
  { frequency: PipelineScheduleFrequency.WEEKLY, label: "Weekly" },
  { frequency: PipelineScheduleFrequency.MONTHLY, label: "Monthly" },
  { frequency: PipelineScheduleFrequency.CUSTOM, label: "Custom" },
];

export const PIPELINE_SCHEDULE_CRON_FIELD_BOUNDS: [number, number][] = [
  [0, 59],
  [0, 23],
  [1, 31],
  [1, 12],
  [0, 7],
];

export const PIPELINE_SCHEDULE_CRON_PART_PATTERN = /^(\*|\d+(?:-\d+)?)(?:\/(\d+))?$/;

export const PIPELINE_SCHEDULE_DAY_OPTIONS: SelectOption[] = [
  { id: "1", label: "Monday" },
  { id: "2", label: "Tuesday" },
  { id: "3", label: "Wednesday" },
  { id: "4", label: "Thursday" },
  { id: "5", label: "Friday" },
  { id: "6", label: "Saturday" },
  { id: "0", label: "Sunday" },
];

export const PIPELINE_SCHEDULE_HOUR_OPTIONS: SelectOption[] = Array.from(
  { length: 24 },
  (_, hour) => ({
    id: `${hour}`,
    label: `${String(hour).padStart(2, "0")}:00`,
  }),
);

const PIPELINE_SCHEDULE_ORDINAL_SUFFIX_MAP: Record<number, string> = {
  1: "st",
  2: "nd",
  3: "rd",
  21: "st",
  22: "nd",
  23: "rd",
};

export const PIPELINE_SCHEDULE_DAY_OF_MONTH_COUNT = 28;

export const PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS: SelectOption[] = Array.from(
  { length: PIPELINE_SCHEDULE_DAY_OF_MONTH_COUNT },
  (_, index) => ({
    id: `${index + 1}`,
    label: `${index + 1}${PIPELINE_SCHEDULE_ORDINAL_SUFFIX_MAP[index + 1] ?? "th"}`,
  }),
);
