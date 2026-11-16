import { useMemo } from "react";

import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { useConnectionWebhookSpec } from "@/components/webhooks/hooks/useConnectionWebhookSpec";
import { isWebhookSplittingExecution } from "@/components/webhooks/utils";

import { useFilamentSearchUpdate, usePipelineParams, usePipelineSearch } from "@/module/hooks";
import type { PipelineSearch } from "@/module/schemas";

import { createGetConnectionInput, useGetConnectionQuery } from "@/api/queries/connections";
import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { isContinuousRun } from "@/utils/runs";

export const usePipelineExecutionLane = () => {
  const { id } = usePipelineParams();
  const { mode } = usePipelineSearch();
  const updateSearch = useFilamentSearchUpdate<PipelineSearch>();
  const { data } = useSuspenseGetPipelineQuery({ input: createGetPipelineInput(id) });
  const graph = data.pipeline?.currentVersion?.graph;
  const sourceConnectionId =
    graph?.nodes.find((node) => node.kind === ConnectorKind.SOURCE)?.connectionId ?? "";
  const { data: sourceData } = useGetConnectionQuery({
    input: createGetConnectionInput(sourceConnectionId),
    options: { enabled: sourceConnectionId !== "" },
  });
  const { webhookSpec } = useConnectionWebhookSpec(sourceData?.connection);
  const pipelineMode =
    data.pipeline?.executionMode === ExecutionMode.CONTINUOUS
      ? ExecutionMode.CONTINUOUS
      : ExecutionMode.BOUNDED;

  return useMemo(() => {
    const webhookResources = isWebhookSplittingExecution(webhookSpec) ? webhookSpec.resources : [];
    const isSplit = pipelineMode === ExecutionMode.BOUNDED && webhookResources.length > 0;
    const hasStreamResources = (graph?.edges ?? []).some((edge) =>
      webhookResources.includes(edge.resource),
    );
    const lane =
      isSplit && mode === ExecutionMode.CONTINUOUS ? ExecutionMode.CONTINUOUS : pipelineMode;
    const isStreamLane = lane === ExecutionMode.CONTINUOUS;
    return {
      isSplit,
      hasStreamResources,
      lane,
      isStreamLane,
      isResourceInLane: (resource: string) =>
        !isSplit || webhookResources.includes(resource) === isStreamLane,
      isRunInLane: (run: RunInfo) => !isSplit || isContinuousRun(run) === isStreamLane,
      setLane: (next: ExecutionMode) =>
        void updateSearch(
          (prev) => ({
            ...prev,
            mode: next === ExecutionMode.CONTINUOUS ? ExecutionMode.CONTINUOUS : undefined,
          }),
          { replace: true },
        ),
    };
  }, [webhookSpec, pipelineMode, graph, mode, updateSearch]);
};
