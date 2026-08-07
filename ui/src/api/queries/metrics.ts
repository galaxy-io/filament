import type { Transport } from "@connectrpc/connect";
import { createConnectQueryKey, type UseQueryOptions, useQuery } from "@connectrpc/connect-query";

import {
  MetricsService,
  type QueryAggregateRequest,
  type QueryAggregateResponse,
  type QueryTimeseriesRequest,
  type QueryTimeseriesResponse,
} from "@/gen/metrics/v1/metrics_pb";

export const METRICS_REFETCH_INTERVAL = 30 * 1000;

export const createQueryTimeseriesQueryKey = (
  input?: QueryTimeseriesRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: MetricsService.method.queryTimeseries,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useQueryTimeseriesQuery = ({
  input,
  options = {},
}: {
  input?: QueryTimeseriesRequest;
  options?: UseQueryOptions<
    typeof MetricsService.method.queryTimeseries.output,
    QueryTimeseriesResponse
  >;
} = {}) => {
  return useQuery<
    typeof MetricsService.method.queryTimeseries.input,
    typeof MetricsService.method.queryTimeseries.output
  >(MetricsService.method.queryTimeseries, input, {
    refetchInterval: METRICS_REFETCH_INTERVAL,
    ...options,
  });
};

export const createQueryAggregateQueryKey = (
  input?: QueryAggregateRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: MetricsService.method.queryAggregate,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useQueryAggregateQuery = ({
  input,
  options = {},
}: {
  input?: QueryAggregateRequest;
  options?: UseQueryOptions<
    typeof MetricsService.method.queryAggregate.output,
    QueryAggregateResponse
  >;
} = {}) => {
  return useQuery<
    typeof MetricsService.method.queryAggregate.input,
    typeof MetricsService.method.queryAggregate.output
  >(MetricsService.method.queryAggregate, input, {
    refetchInterval: METRICS_REFETCH_INTERVAL,
    ...options,
  });
};
