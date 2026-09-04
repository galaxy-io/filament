import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import PipelineHistoryPage from "@/pages/pipelines/PipelineHistoryPage";

const searchParams = z.object({
  runId: z.array(z.string()).optional().catch(undefined),
});

export const Route = createFileRoute("/_app/pipelines/$id/history")({
  validateSearch: searchParams,
  component: PipelineHistoryPage,
});
