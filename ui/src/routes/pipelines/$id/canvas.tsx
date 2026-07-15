import { createFileRoute, useParams } from "@tanstack/react-router";

import PipelineCanvas from "@/pages/pipelines/canvas/PipelineCanvas";

const PipelineCanvasRoute = () => {
  const { id } = useParams({ from: "/pipelines/$id/canvas" });

  return <PipelineCanvas pipelineId={id} />;
};

export const Route = createFileRoute("/pipelines/$id/canvas")({
  component: PipelineCanvasRoute,
});
