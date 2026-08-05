import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import ObservabilityPage from "@/pages/observability/ObservabilityPage";
import { ObservabilityTimeframe } from "@/pages/observability/types";

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
});

export const Route = createFileRoute("/_main/observability")({
  validateSearch: searchParams,
  component: ObservabilityPage,
});
