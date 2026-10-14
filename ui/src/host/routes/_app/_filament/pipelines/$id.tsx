import { createFileRoute } from "@tanstack/react-router";

import PipelinePage from "@/pages/pipelines/PipelinePage";

import { pipelineRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/pipelines/$id")({
  ...pipelineRouteOptions,
  component: PipelinePage,
});
