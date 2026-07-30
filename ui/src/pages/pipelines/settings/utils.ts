import { CronExpressionParser } from "cron-parser";

import type { PipelineSchedule } from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_SCHEDULE_DEFAULT_TIMEZONE } from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/PipelineSettingsPageSchedule";

import { getErrorMessage } from "@/utils/errors";

export const getPipelineScheduleCronError = (value: string): string | null => {
  const cron = value.trim();
  try {
    CronExpressionParser.parse(cron);
    return null;
  } catch (error) {
    return getErrorMessage(error, "Invalid cron expression");
  }
};

export const mapPipelineScheduleCronToCanonical = (value: string): string => {
  return CronExpressionParser.parse(value.trim()).stringify();
};

export const hasPipelineScheduleChanges = (
  state: PipelineSettingsPageScheduleState,
  schedule?: PipelineSchedule,
): boolean => {
  if (!schedule?.config) return true;
  return (
    state.enabled !== schedule.config.enabled ||
    state.cron.trim() !== schedule.config.cron ||
    state.timezone !== (schedule.config.timezone || PIPELINE_SCHEDULE_DEFAULT_TIMEZONE)
  );
};
