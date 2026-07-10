import { createFileRoute } from "@tanstack/react-router";

import PipelineHistoryPage from "@/pages/pipelines/PipelineHistoryPage";

export const Route = createFileRoute("/pipelines/$id/history")({
  component: PipelineHistoryPage,
});
