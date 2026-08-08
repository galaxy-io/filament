import type { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { ResourceColumn } from "@/gen/ingestion/v1/providers_pb";

import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

export enum CreatePipelineModalStep {
  CONNECTIONS = "CONNECTIONS",
  RESOURCES = "RESOURCES",
  DELIVERY = "DELIVERY",
  DETAILS = "DETAILS",
}

export interface CreatePipelineModalState {
  step: CreatePipelineModalStep;
  activeSinkId: string;
  sourceConnection: Connection | null;
  sinkConnections: Connection[];
  resourceSelection: Record<string, Record<string, boolean>>;
  resourceReadModes: Record<string, Record<string, ReadMode>>;
  resourceCursors: Record<string, Record<string, string>>;
  sinkWriteModes: Record<string, WriteMode>;
  name: string;
  isNameTouched: boolean;
  description: string;
  schedule: PipelineSettingsPageScheduleState;
  isSubmitting: boolean;
}

export interface CreatePipelineModalResourceStatus {
  message: string;
  isBlocking: boolean;
}

export interface CreatePipelineModalResourceRow {
  name: string;
  displayName: string;
  isSelectable: boolean;
  isSelected: boolean;
  readMode: ReadMode;
  readModeOptions: ReadMode[];
  cursorField: string;
  cursorOptions: ResourceColumn[];
  status?: CreatePipelineModalResourceStatus;
}

export interface CreatePipelineModalSinkRow {
  connection: Connection;
  writeMode: WriteMode;
  writeModeOptions: WriteMode[];
}
