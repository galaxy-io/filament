import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunPipelineRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import { useRunPipelineMutation } from "@/api/queries/runs";

import { NOOP } from "@/constants";

import { getErrorMessage } from "@/utils/errors";

interface PipelineRunOptions {
  workerConfiguration?: WorkerConfiguration;
  resources?: string[];
  resyncResources?: string[];
  onSuccess?: () => void;
}

export const usePipelineRun = () => {
  const { toast } = useToast();
  const { mutateAsync: runPipeline, isPending: isRunning } = useRunPipelineMutation();

  const startRun = useCallback(
    (
      pipeline: Pipeline,
      { workerConfiguration, resources, resyncResources, onSuccess }: PipelineRunOptions = {},
    ) => {
      const run = runPipeline(
        create(RunPipelineRequestSchema, {
          pipelineId: pipeline.id,
          workerConfiguration,
          resources,
          resyncResources,
          options: { executionMode: pipeline.executionMode },
        }),
      );
      toast.promise(run, {
        loading: "Starting run...",
        success: { header: "Run started", variant: ToastVariant.SUCCESS },
        error: (error) => ({
          header: "Start failed",
          description: getErrorMessage(error, "Failed to run pipeline"),
          variant: ToastVariant.ERROR,
        }),
      });
      void run.then(() => onSuccess?.(), NOOP);
    },
    [runPipeline, toast],
  );

  return { startRun, isRunning };
};
