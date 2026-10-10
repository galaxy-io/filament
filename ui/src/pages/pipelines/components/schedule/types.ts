import type { PipelineScheduleConfig } from "@/gen/ingestion/v1/pipelines_pb";

export interface PipelineScheduleState {
  isEnabled: boolean;
  cron: PipelineScheduleConfig["cron"];
  timezone: PipelineScheduleConfig["timezone"];
}
