import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

export const PIPELINE_SCHEDULE_FIELDS_COLUMNS = "repeat(2, minmax(0, 1fr))";

export const PIPELINE_SCHEDULE_DEFAULT_TIMEZONE = Intl.DateTimeFormat().resolvedOptions().timeZone;

export const PIPELINE_SCHEDULE_DEFAULT_STATE: PipelineSettingsPageScheduleState = {
  isEnabled: false,
  cron: "0 9 * * *",
  timezone: PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
};

export const PIPELINE_SCHEDULE_TIMEZONE_OPTIONS: SelectOption[] = Intl.supportedValuesOf(
  "timeZone",
).map((timezone) => ({
  id: timezone,
  label: timezone,
}));

export const PIPELINE_SCHEDULE_CRON_FIELD_BOUNDS: [number, number][] = [
  [0, 59],
  [0, 23],
  [1, 31],
  [1, 12],
  [0, 7],
];

export const PIPELINE_SCHEDULE_CRON_PART_PATTERN = /^(\*|\d+(?:-\d+)?)(?:\/(\d+))?$/;
