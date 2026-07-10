import { createFileRoute } from "@tanstack/react-router";

import PipelinePage from "@/pages/pipelines/PipelinePage";

export const Route = createFileRoute("/pipelines/$id")({
  component: PipelinePage,
});
