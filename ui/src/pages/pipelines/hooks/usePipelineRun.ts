import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunPipelineRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import { formatPipelineName } from "@/components/pipelines/utils";

import { useRunPipelineMutation } from "@/api/queries/runs";

import { getErrorMessage } from "@/utils/errors";

interface PipelineRunOptions {
  workerConfiguration?: WorkerConfiguration;
  onSuccess?: () => void;
}

export const usePipelineRun = () => {
  const { toast } = useToast();
  const { mutate: runPipeline, isPending: isRunning } = useRunPipelineMutation();

  const startRun = useCallback(
    (pipeline: Pipeline, { workerConfiguration, onSuccess }: PipelineRunOptions = {}) =>
      runPipeline(
        create(RunPipelineRequestSchema, {
          pipelineId: pipeline.id,
          workerConfiguration,
          options: { executionMode: pipeline.executionMode },
        }),
        {
          onSuccess: () => {
            onSuccess?.();
            toast({
              header: "Run started",
              description: `${formatPipelineName(pipeline)} is now running.`,
              variant: ToastVariant.SUCCESS,
            });
          },
          onError: (error) => {
            toast({
              header: "Run failed",
              description: getErrorMessage(error, "Failed to run pipeline"),
              variant: ToastVariant.ERROR,
            });
          },
        },
      ),
    [runPipeline, toast],
  );

  return { startRun, isRunning };
};
