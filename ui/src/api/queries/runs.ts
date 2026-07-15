import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  UseMutationOptions,
  UseQueryOptions,
} from "@connectrpc/connect-query";

import {
  GetRunRequest,
  GetRunResponse,
  ListRunsRequest,
  ListRunsResponse,
  RunStatus,
} from "@/gen/ingestion/v1/runs_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

const listRuns = IngestionService.method.listRuns;
const getRun = IngestionService.method.getRun;
const runPipeline = IngestionService.method.runPipeline;
const signalRun = IngestionService.method.signalRun;

const ACTIVE_RUN_STATUSES = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
  RunStatus.PAUSED,
]);

// ========== LIST RUNS ==========

export const createListRunsQueryKey = (input?: ListRunsRequest) => {
  return createConnectQueryKey({
    schema: listRuns,
    input,
    cardinality: "finite",
  });
};

export const useListRunsQuery = ({
  input,
  options = {},
}: {
  input?: ListRunsRequest;
  options?: UseQueryOptions<typeof listRuns.output, ListRunsResponse>;
} = {}) => {
  return useQuery<typeof listRuns.input, typeof listRuns.output>(listRuns, input, {
    refetchInterval: (query) => {
      const runs = query.state.data?.runs;
      // Poll while any run is still in flight.
      const hasActiveRun = runs?.some((run) => ACTIVE_RUN_STATUSES.has(run.status));
      return hasActiveRun ? 3 * 1000 : false;
    },
    ...options,
  });
};

// ========== GET RUN ==========

export const createGetRunQueryKey = (input: GetRunRequest) => {
  return createConnectQueryKey({
    schema: getRun,
    input,
    cardinality: "finite",
  });
};

export const useGetRunQuery = ({
  input,
  options = {},
}: {
  input: GetRunRequest;
  options?: UseQueryOptions<typeof getRun.output, GetRunResponse>;
}) => {
  return useQuery<typeof getRun.input, typeof getRun.output>(getRun, input, {
    refetchInterval: (query) => {
      const status = query.state.data?.snapshot?.run?.status;
      return status !== undefined && ACTIVE_RUN_STATUSES.has(status) ? 2 * 1000 : false;
    },
    ...options,
  });
};

// ========== MUTATIONS ==========

export const useRunPipelineMutation = (
  options: UseMutationOptions<typeof runPipeline.input, typeof runPipeline.output> = {},
) => {
  return useMutation(runPipeline, options);
};

export const useSignalRunMutation = (
  options: UseMutationOptions<typeof signalRun.input, typeof signalRun.output> = {},
) => {
  return useMutation(signalRun, options);
};
