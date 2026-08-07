import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import {
  OBSERVABILITY_SETUP_CONNECTIONS_INPUT,
  OBSERVABILITY_SETUP_PIPELINES_INPUT,
} from "@/pages/observability/constants";
import ObservabilityPage from "@/pages/observability/ObservabilityPage";
import { ObservabilityTimeframe } from "@/pages/observability/types";

import { createListConnectionsQueryOptions } from "@/api/queries/connections";
import { createListPipelinesQueryOptions } from "@/api/queries/pipelines";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const pivotDimension = z.union([
  z.literal(MetricDimension.PIPELINE_ID),
  z.literal(MetricDimension.STATUS),
]);

export type ObservabilityPivotDimension = z.infer<typeof pivotDimension>;

const searchParams = z.object({
  timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
  records: pivotDimension.optional().catch(undefined),
  volume: pivotDimension.optional().catch(undefined),
  statuses: z.array(z.number()).optional().catch(undefined),
  refreshedAt: z.number().optional().catch(undefined),
});

export const Route = createFileRoute("/_main/observability")({
  validateSearch: searchParams,
  loader: () =>
    Promise.all([
      queryClient.ensureQueryData(
        createListConnectionsQueryOptions({
          input: OBSERVABILITY_SETUP_CONNECTIONS_INPUT,
          transport,
        }),
      ),
      queryClient.ensureQueryData(
        createListPipelinesQueryOptions({
          input: OBSERVABILITY_SETUP_PIPELINES_INPUT,
          transport,
        }),
      ),
    ]),
  component: ObservabilityPage,
});
