import type {
  ReplicationMode,
  StandardSyncMode,
  WorkerResources,
} from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

export enum CreatePipelineModalStep {
  CONNECTIONS = "CONNECTIONS",
  RESOURCES = "RESOURCES",
  DELIVERY = "DELIVERY",
  DETAILS = "DETAILS",
}

export enum CreatePipelineModalStepStatus {
  COMPLETED = "COMPLETED",
  CURRENT = "CURRENT",
  UPCOMING = "UPCOMING",
}

export interface CreatePipelineModalState {
  step: CreatePipelineModalStep;
  activeSinkId: Connection["id"];
  sourceConnection: Connection | null;
  sinkConnections: Connection[];
  resourceSelection: Record<Connection["id"], Record<Resource["name"], boolean>>;
  resourceSyncModes: Record<Connection["id"], Record<Resource["name"], StandardSyncMode>>;
  resourceCursors: Record<Connection["id"], Record<Resource["name"], ResourceColumn["name"]>>;
  name: Pipeline["name"];
  isNameTouched: boolean;
  description: Pipeline["description"];
  schedule: PipelineSettingsPageScheduleState;
  workerResources: WorkerResources;
  isSubmitting: boolean;
}

export interface CreatePipelineModalResourceStatus {
  message: string;
  isBlocking: boolean;
}

export interface CreatePipelineModalResourceRow {
  name: Resource["name"];
  displayName: Resource["displayName"];
  isSelectable: boolean;
  isSelected: boolean;
  syncMode: StandardSyncMode;
  syncModeOptions: StandardSyncMode[];
  cursorField: ResourceColumn["name"];
  cursorOptions: ResourceColumn[];
  status?: CreatePipelineModalResourceStatus;
}

export interface CreatePipelineModalSinkRow {
  connection: Connection;
}

export interface CreatePipelineModalDerivedState {
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>;
  sinks: CreatePipelineModalSinkRow[];
  replication: ReplicationMode;
  isCdc: boolean;
  issuesBySink: Record<Connection["id"], string[]>;
  selectedCountBySink: Record<Connection["id"], number>;
  isLoading: boolean;
  discoverError?: Error | null;
  effectiveName: Pipeline["name"];
  nameError?: string;
  isNextDisabled: boolean;
  hints: string[];
  stepIndex: number;
  isBackVisible: boolean;
  isLastStep: boolean;
}

export type CreatePipelineModalContextValue = CreatePipelineModalState &
  CreatePipelineModalDerivedState;
