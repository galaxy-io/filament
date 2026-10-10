import { createFileRoute } from "@tanstack/react-router";

import PipelineHistoryPage from "@/pages/pipelines/PipelineHistoryPage";

import { pipelineHistorySearchSchema } from "@/module/schemas";

export const Route = createFileRoute("/_app/pipelines/$id/history")({
  validateSearch: pipelineHistorySearchSchema,
  component: PipelineHistoryPage,
});
