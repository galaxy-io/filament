import z from "zod";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { SortBy, SortOrder } from "@/gen/ingestion/v1/sorting_pb";
import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import {
  ObservabilityRunsView,
  ObservabilityThroughputView,
  ObservabilityTimeframe,
  ObservabilityUsageView,
} from "@/pages/observability/types";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import { PipelineCanvasView } from "@/pages/pipelines/canvas/types";
import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { Flow } from "@/module/types";

export const listSearchParamsSchema = z.object({
  q: z.string().optional().catch(undefined),
  sortBy: z.enum(SortBy).optional().catch(undefined),
  sortOrder: z.enum(SortOrder).optional().catch(undefined),
});

export type ListSearchParams = z.infer<typeof listSearchParamsSchema>;

export const filamentLayoutSearchSchema = z.object({
  flow: z.enum(Flow).optional().catch(undefined),
  connectionId: z.string().optional().catch(undefined),
  connectorKind: z.enum(ConnectorKind).optional().catch(undefined),
  connector: z.string().optional().catch(undefined),
  connectorSearch: z.string().optional().catch(undefined),
});

export type FilamentLayoutSearch = z.infer<typeof filamentLayoutSearchSchema>;

export const settingsSearchSchema = z.object({
  settings: z.enum(SettingsPanel).optional().catch(undefined),
  teamView: z.enum(TeamSettingsView).optional().catch(undefined),
  inviteToken: z.string().optional().catch(undefined),
});

export type SettingsSearch = z.infer<typeof settingsSearchSchema>;

export const observabilitySearchSchema = listSearchParamsSchema
  .pick({ sortBy: true, sortOrder: true })
  .extend({
    timeframe: z.enum(ObservabilityTimeframe).optional().catch(undefined),
    runs: z.enum(ObservabilityRunsView).optional().catch(undefined),
    throughput: z.enum(ObservabilityThroughputView).optional().catch(undefined),
    throughputPivot: z.enum(MetricDimension).optional().catch(undefined),
    usage: z.enum(ObservabilityUsageView).optional().catch(undefined),
    usagePivot: z.enum(MetricDimension).optional().catch(undefined),
    statuses: z.array(z.enum(RunStatus)).optional().catch(undefined),
    runsBucket: z.coerce.bigint().positive().optional().catch(undefined),
    runsStatus: z.enum(RunStatus).optional().catch(undefined),
  });

export type ObservabilitySearch = z.infer<typeof observabilitySearchSchema>;

export const pipelinesSearchSchema = listSearchParamsSchema;

export type PipelinesSearch = z.infer<typeof pipelinesSearchSchema>;

export const connectionsSearchSchema = listSearchParamsSchema.pick({ q: true });

export type ConnectionsSearch = z.infer<typeof connectionsSearchSchema>;

export const pipelineParamsSchema = z.object({ id: z.string() });

export const pipelineSearchSchema = z.object({
  version: z.coerce.bigint().positive().optional().catch(undefined),
});

export type PipelineSearch = z.infer<typeof pipelineSearchSchema>;

export const pipelineCanvasSearchSchema = z.object({
  node: z.string().optional().catch(undefined),
  resource: z.string().optional().catch(undefined),
  showPanel: z.boolean().optional().catch(undefined),
  tab: z.enum(PipelineCanvasPanelTab).optional().catch(undefined),
  view: z.enum(PipelineCanvasView).optional().catch(undefined),
  sinks: z.array(z.string()).optional().catch(undefined),
});

export type PipelineCanvasSearch = z.infer<typeof pipelineCanvasSearchSchema>;

export const pipelineHistorySearchSchema = z.object({
  runId: z.array(z.string()).optional().catch(undefined),
});

export type PipelineHistorySearch = z.infer<typeof pipelineHistorySearchSchema>;
