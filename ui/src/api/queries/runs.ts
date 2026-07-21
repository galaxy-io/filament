import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useTransport,
} from "@connectrpc/connect-query";

import {
  type GetRunRequest,
  type GetRunResponse,
  type ListRunsRequest,
  type ListRunsResponse,
  type RunEvent,
  RunStatus,
  TailRunRequestSchema,
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

// ========== TAIL RUNS (server streaming) ==========

const MAX_TAIL_EVENTS = 2000;
const TAIL_FLUSH_INTERVAL_MS = 400;

/**
 * Tails one or more runs via the TailRun server stream into a single merged feed.
 * Reconnects (with snapshot replay) whenever the run ids change; aborts on unmount.
 * Events are flushed in batches so a chatty run doesn't cause a render per fact.
 */
export const useTailRunsStream = (runIds: string[]) => {
  const transport = useTransport();
  const client = useMemo(() => createClient(IngestionService, transport), [transport]);

  const [events, setEvents] = useState<RunEvent[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  // Key by content so a re-created array with the same ids doesn't reconnect
  const runIdsKey = runIds.join("|");

  const connect = useCallback(async () => {
    abortRef.current?.abort();
    setEvents([]);

    const ids = runIdsKey ? runIdsKey.split("|") : [];
    if (ids.length === 0) {
      setIsStreaming(false);
      return;
    }

    const abortController = new AbortController();
    abortRef.current = abortController;

    const buffer: RunEvent[] = [];
    let flushTimer: number | null = null;

    const flushBuffer = () => {
      flushTimer = null;
      if (buffer.length === 0 || abortController.signal.aborted) return;
      const batch = buffer.splice(0, buffer.length);
      setEvents((prev) => {
        const merged = [...prev, ...batch];
        return merged.length > MAX_TAIL_EVENTS
          ? merged.slice(merged.length - MAX_TAIL_EVENTS)
          : merged;
      });
    };

    const scheduleFlush = () => {
      if (flushTimer === null) {
        flushTimer = window.setTimeout(flushBuffer, TAIL_FLUSH_INTERVAL_MS);
      }
    };

    setIsStreaming(true);
    try {
      await Promise.all(
        ids.map(async (runId) => {
          for await (const response of client.tailRun(
            create(TailRunRequestSchema, { runId, replay: true }),
            { signal: abortController.signal },
          )) {
            if (!response.event) continue;
            buffer.push(response.event);
            scheduleFlush();
          }
        }),
      );
    } catch (error) {
      if (!abortController.signal.aborted) {
        console.error("[TailRuns] stream error:", error);
      }
    } finally {
      if (flushTimer !== null) {
        window.clearTimeout(flushTimer);
      }
      flushBuffer();
      if (!abortController.signal.aborted) {
        setIsStreaming(false);
      }
    }
  }, [client, runIdsKey]);

  useEffect(() => {
    void connect();
    return () => {
      abortRef.current?.abort();
    };
  }, [connect]);

  const clear = useCallback(() => {
    setEvents([]);
  }, []);

  return { events, isStreaming, clear };
};
