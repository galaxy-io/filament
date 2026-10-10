import { createFileRoute } from "@tanstack/react-router";

import PipelineCanvasPage from "@/pages/pipelines/PipelineCanvasPage";

import { pipelineCanvasRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/pipelines/$id/canvas")({
  ...pipelineCanvasRouteOptions,
  component: PipelineCanvasPage,
});
