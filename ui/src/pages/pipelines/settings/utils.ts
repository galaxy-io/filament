import type { PipelineSchedule } from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS,
  PIPELINE_SCHEDULE_DAY_OPTIONS,
  PIPELINE_SCHEDULE_DEFAULT_TIMEZONE,
} from "@/pages/pipelines/settings/constants";
import {
  PipelineScheduleFrequency,
  type PipelineSettingsPageScheduleState,
} from "@/pages/pipelines/settings/types";

export const mapPipelineScheduleStateToCron = (
  state: PipelineSettingsPageScheduleState,
): string => {
  if (state.frequency === PipelineScheduleFrequency.HOURLY) return "0 * * * *";
  if (state.frequency === PipelineScheduleFrequency.WEEKLY) {
    return `0 ${state.hour} * * ${[...state.days].sort((a, b) => a - b).join(",")}`;
  }
  if (state.frequency === PipelineScheduleFrequency.MONTHLY) {
    return `0 ${state.hour} ${state.dayOfMonth} * *`;
  }
  return `0 ${state.hour} * * *`;
};

type PipelineScheduleCronState = Pick<
  PipelineSettingsPageScheduleState,
  "frequency" | "days" | "dayOfMonth" | "hour"
>;

export const mapPipelineScheduleCronToState = (
  cron: string,
): Partial<PipelineScheduleCronState> | null => {
  const fields = cron.trim().split(/\s+/);
  if (fields.length !== 5) return null;
  const [minute, hour, dayOfMonth, month, dayOfWeek] = fields;
  if (minute !== "0" || month !== "*") return null;
  if (hour === "*") {
    if (dayOfMonth !== "*" || dayOfWeek !== "*") return null;
    return { frequency: PipelineScheduleFrequency.HOURLY };
  }
  const hourValue = Number(hour);
  if (!Number.isInteger(hourValue) || hourValue < 0 || hourValue > 23) return null;
  if (dayOfWeek !== "*") {
    if (dayOfMonth !== "*") return null;
    const days = dayOfWeek.split(",").map(Number);
    if (days.some((day) => !Number.isInteger(day) || day < 0 || day > 6)) return null;
    return {
      frequency: PipelineScheduleFrequency.WEEKLY,
      hour: hourValue,
      days,
    };
  }
  if (dayOfMonth !== "*") {
    const dayValue = Number(dayOfMonth);
    if (!Number.isInteger(dayValue) || dayValue < 1 || dayValue > 28) return null;
    return {
      frequency: PipelineScheduleFrequency.MONTHLY,
      hour: hourValue,
      dayOfMonth: dayValue,
    };
  }
  return { frequency: PipelineScheduleFrequency.DAILY, hour: hourValue };
};

export const formatPipelineScheduleSummary = (
  state: PipelineSettingsPageScheduleState,
): string | null => {
  if (state.frequency === PipelineScheduleFrequency.HOURLY) return "Runs every hour";
  const time = `${String(state.hour).padStart(2, "0")}:00 ${state.timezone}`;
  if (state.frequency === PipelineScheduleFrequency.WEEKLY) {
    const labels = PIPELINE_SCHEDULE_DAY_OPTIONS.filter((option) =>
      state.days.includes(option.value as number),
    ).map((option) => option.label);
    if (labels.length === 0) return null;
    if (labels.length === 7) return `Runs every day at ${time}`;
    const joined =
      labels.length === 1
        ? labels[0]
        : `${labels.slice(0, -1).join(", ")} and ${labels[labels.length - 1]}`;
    return `Runs every ${joined} at ${time}`;
  }
  if (state.frequency === PipelineScheduleFrequency.MONTHLY) {
    const dayLabel = PIPELINE_SCHEDULE_DAY_OF_MONTH_OPTIONS.find(
      (option) => option.value === state.dayOfMonth,
    )?.label;
    return `Runs on the ${dayLabel} of every month at ${time}`;
  }
  return `Runs every day at ${time}`;
};

export const hasPipelineScheduleChanges = (
  state: PipelineSettingsPageScheduleState,
  schedule?: PipelineSchedule,
): boolean => {
  if (!schedule?.config) return true;
  return (
    state.isEnabled !== schedule.config.enabled ||
    mapPipelineScheduleStateToCron(state) !== schedule.config.cron ||
    state.timezone !== (schedule.config.timezone || PIPELINE_SCHEDULE_DEFAULT_TIMEZONE)
  );
};
