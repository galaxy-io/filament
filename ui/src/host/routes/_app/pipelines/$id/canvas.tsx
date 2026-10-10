import { createFileRoute } from "@tanstack/react-router";

import PipelineCanvasPage from "@/pages/pipelines/PipelineCanvasPage";

import { pipelineCanvasSearchSchema } from "@/module/schemas";

export const Route = createFileRoute("/_app/pipelines/$id/canvas")({
  validateSearch: pipelineCanvasSearchSchema,
  component: PipelineCanvasPage,
});
