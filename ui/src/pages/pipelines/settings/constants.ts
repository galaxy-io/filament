import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  PipelineScheduleFrequency,
  type PipelineScheduleFrequencyOption,
} from "@/pages/pipelines/settings/types";

export const PIPELINE_SCHEDULE_DEFAULT_TIMEZONE = Intl.DateTimeFormat().resolvedOptions().timeZone;

export const PIPELINE_SCHEDULE_TIMEZONE_OPTIONS: SelectInputOption[] = Intl.supportedValuesOf(
  "timeZone",
).map((timezone) => ({
  id: timezone,
  label: timezone,
  value: timezone,
}));

export const PIPELINE_SCHEDULE_FREQUENCY_OPTIONS: PipelineScheduleFrequencyOption[] = [
  { frequency: PipelineScheduleFrequency.HOURLY, label: "Hourly" },
  { frequency: PipelineScheduleFrequency.DAILY, label: "Daily" },
  { frequency: PipelineScheduleFrequency.WEEKLY, label: "Weekly" },
  { frequency: PipelineScheduleFrequency.MONTHLY, label: "Monthly" },
];

export const PIPELINE_SCHEDULE_DAY_OPTIONS: SelectInputOption[] = [
  { id: "1", label: "Monday", value: 1 },
  { id: "2", label: "Tuesday", value: 2 },
  { id: "3", label: "Wednesday", value: 3 },
  { id: "4", label: "Thursday", value: 4 },
  { id: "5", label: "Friday", value: 5 },
  { id: "6", label: "Saturday", value: 6 },
  { id: "0", label: "Sunday", value: 0 },
];

export const PIPELINE_SCHEDULE_HOUR_OPTIONS: SelectInputOption[] = Array.from(
  { length: 24 },
  (_, hour) => ({
    id: `${hour}`,
    label: `${String(hour).padStart(2, "0")}:00`,
    value: hour,
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

export const PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS: SelectInputOption[] = Array.from(
  { length: 28 },
  (_, index) => ({
    id: `${index + 1}`,
    label: `${index + 1}${PIPELINE_SCHEDULE_ORDINAL_SUFFIX_MAP[index + 1] ?? "th"}`,
    value: index + 1,
  }),
);
