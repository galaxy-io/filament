import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

export const PIPELINE_SCHEDULE_DEFAULT_TIMEZONE = Intl.DateTimeFormat().resolvedOptions().timeZone;

export const PIPELINE_SCHEDULE_TIMEZONE_OPTIONS: SelectInputOption[] = Intl.supportedValuesOf(
  "timeZone",
).map((timezone) => ({
  id: timezone,
  label: timezone,
  value: timezone,
}));
