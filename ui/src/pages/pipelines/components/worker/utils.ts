import { create } from "@bufbuild/protobuf";

import {
  type WorkerConfiguration,
  WorkerConfigurationSchema,
  WorkerResourcesSchema,
} from "@/gen/ingestion/v1/common_pb";

import type {
  WorkerResourcesDraft,
  WorkerResourcesState,
} from "@/pages/pipelines/components/worker/types";

export const trimWorkerResourceValue = (value: string | undefined) => value?.trim() ?? "";

export const trimWorkerResourcesState = (state: WorkerResourcesDraft): WorkerResourcesState => ({
  cpuRequest: trimWorkerResourceValue(state?.cpuRequest),
  cpuLimit: trimWorkerResourceValue(state?.cpuLimit),
  memoryRequest: trimWorkerResourceValue(state?.memoryRequest),
  memoryLimit: trimWorkerResourceValue(state?.memoryLimit),
});

export const hasWorkerResourcesStateValues = (state: WorkerResourcesDraft) =>
  Object.values(trimWorkerResourcesState(state)).some(Boolean);

export const mapWorkerResourcesStateToWorkerConfiguration = (
  state: WorkerResourcesDraft,
): WorkerConfiguration | undefined => {
  const resources = trimWorkerResourcesState(state);
  if (!hasWorkerResourcesStateValues(resources)) return undefined;

  return create(WorkerConfigurationSchema, {
    resources: create(WorkerResourcesSchema, resources),
  });
};
