import type { PipelineScheduleConfig } from "@/gen/ingestion/v1/pipelines_pb";

export enum PipelineScheduleFrequency {
  HOURLY = "HOURLY",
  DAILY = "DAILY",
  WEEKLY = "WEEKLY",
  MONTHLY = "MONTHLY",
  CUSTOM = "CUSTOM",
}

export interface PipelineScheduleFrequencyOption {
  frequency: PipelineScheduleFrequency;
  label: string;
}

export interface PipelineSettingsPageScheduleState {
  isEnabled: boolean;
  frequency: PipelineScheduleFrequency;
  days: number[];
  dayOfMonth: number;
  hour: number;
  cron: PipelineScheduleConfig["cron"];
  timezone: PipelineScheduleConfig["timezone"];
}
