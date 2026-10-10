import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { CREATE_PIPELINE_MODAL_DEFAULT_CRON } from "@/pages/pipelines/components/create/constants";
import {
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/components/schedule/constants";
import { PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT } from "@/pages/pipelines/components/worker/constants";

export const createInitialCreatePipelineModalState = (): CreatePipelineModalState => ({
  executionMode: ExecutionMode.BOUNDED,
  manualResources: [],
  step: CreatePipelineModalStep.CONNECTIONS,
  activeSinkId: "",
  sourceConnection: null,
  sinkConnections: [],
  resourceSelection: {},
  resourceReadModes: {},
  resourceCursors: {},
  sinkWriteModes: {},
  nodeConfigs: {},
  name: "",
  isNameTouched: false,
  description: "",
  schedule: {
    ...PIPELINE_SCHEDULE_DEFAULT_STATE,
    isEnabled: true,
    cron: CREATE_PIPELINE_MODAL_DEFAULT_CRON,
  },
  notifiers: [],
  workerConfiguration: PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT,
  isSubmitting: false,
});
