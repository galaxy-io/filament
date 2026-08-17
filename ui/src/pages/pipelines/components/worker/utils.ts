import { create } from "@bufbuild/protobuf";

import {
  type WorkerConfiguration,
  WorkerConfigurationSchema,
  type WorkerResources,
  WorkerResourcesSchema,
} from "@/gen/ingestion/v1/common_pb";

export const mapWorkerResourcesToWorkerConfiguration = (
  state: WorkerResources,
): WorkerConfiguration | undefined => {
  const resources = {
    cpuRequest: state.cpuRequest.trim(),
    cpuLimit: state.cpuLimit.trim(),
    memoryRequest: state.memoryRequest.trim(),
    memoryLimit: state.memoryLimit.trim(),
  };
  if (!Object.values(resources).some(Boolean)) return undefined;

  return create(WorkerConfigurationSchema, {
    resources: create(WorkerResourcesSchema, resources),
  });
};
