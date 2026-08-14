import type { WorkerResourcesState } from "@/pages/pipelines/components/worker/types";

export const WORKER_RESOURCES_DEFAULT_STATE: WorkerResourcesState = {
  cpuRequest: "",
  cpuLimit: "",
  memoryRequest: "",
  memoryLimit: "",
};
