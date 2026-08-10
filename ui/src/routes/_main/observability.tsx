import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import ObservabilityPage from "@/pages/observability/ObservabilityPage";
import {
  ObservabilityMetricView,
  ObservabilityTimeframe,
} from "@/pages/observability/types";
import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

const searchParams = z.object({
  timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
  metric: z.enum(ObservabilityMetricView).optional().catch(undefined),
  pivot: z.enum(MetricDimension).optional().catch(undefined),
  statuses: z.array(z.enum(RunStatus)).optional().catch(undefined),
});

export const Route = createFileRoute("/_main/observability")({
  validateSearch: searchParams,
  component: ObservabilityPage,
});
