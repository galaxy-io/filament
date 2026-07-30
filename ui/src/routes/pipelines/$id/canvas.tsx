import { createFileRoute } from "@tanstack/react-router";

import PipelineCanvasPage from "@/pages/pipelines/PipelineCanvasPage";

export const Route = createFileRoute("/pipelines/$id/canvas")({
  component: PipelineCanvasPage,
});
