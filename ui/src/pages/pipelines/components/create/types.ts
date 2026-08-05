import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

export enum CreatePipelineModalStep {
  CONNECTIONS = "CONNECTIONS",
  DETAILS = "DETAILS",
  SCHEDULE = "SCHEDULE",
}

export interface CreatePipelineModalState {
  step: CreatePipelineModalStep;
  sourceConnection: Connection | null;
  sinkConnections: Connection[];
  name: string;
  isNameTouched: boolean;
  description: string;
  schedule: PipelineSettingsPageScheduleState;
  isSubmitting: boolean;
}
