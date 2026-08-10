import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import PipelineCanvasPage from "@/pages/pipelines/PipelineCanvasPage";

const searchParams = z.object({
  version: z.number().int().positive().optional().catch(undefined),
});

export const Route = createFileRoute("/pipelines/$id/canvas")({
  validateSearch: searchParams,
  component: PipelineCanvasPage,
});
