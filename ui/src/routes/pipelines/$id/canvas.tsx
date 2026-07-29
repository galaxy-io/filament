import { createFileRoute } from "@tanstack/react-router";

import PipelineCanvas from "@/pages/pipelines/PipelineCanvasPage";

export const Route = createFileRoute("/pipelines/$id/canvas")({
  component: PipelineCanvas,
});
