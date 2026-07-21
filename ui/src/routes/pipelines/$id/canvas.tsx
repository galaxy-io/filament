import { createFileRoute } from "@tanstack/react-router";

import PipelineCanvas from "@/pages/pipelines/canvas/PipelineCanvas";

export const Route = createFileRoute("/pipelines/$id/canvas")({
  component: PipelineCanvas,
});
