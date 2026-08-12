import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import ObservabilityPage from "@/pages/observability/ObservabilityPage";
import {
  ObservabilityThroughputView,
  ObservabilityTimeframe,
  ObservabilityUsageView,
} from "@/pages/observability/types";

const searchParams = z.object({
  timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
  throughput: z.enum(ObservabilityThroughputView).optional().catch(undefined),
  throughputPivot: z.enum(MetricDimension).optional().catch(undefined),
  usage: z.enum(ObservabilityUsageView).optional().catch(undefined),
  usagePivot: z.enum(MetricDimension).optional().catch(undefined),
  statuses: z.array(z.enum(RunStatus)).optional().catch(undefined),
});

export const Route = createFileRoute("/_main/observability")({
  validateSearch: searchParams,
  component: ObservabilityPage,
});
