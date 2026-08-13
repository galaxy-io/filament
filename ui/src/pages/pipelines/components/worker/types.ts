import type { WorkerResources } from "@/gen/ingestion/v1/common_pb";

export interface WorkerResourcesState {
  cpuRequest: string;
  cpuLimit: string;
  memoryRequest: string;
  memoryLimit: string;
}

export type WorkerResourcesDraft = Partial<WorkerResourcesState> | WorkerResources | undefined;
