import { createFileRoute } from "@tanstack/react-router";

import PipelineHistoryPage from "@/pages/pipelines/PipelineHistoryPage";

import { pipelineHistoryRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/pipelines/$id/history")({
  ...pipelineHistoryRouteOptions,
  component: PipelineHistoryPage,
});
