import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import PipelineCanvasPage from "@/pages/pipelines/PipelineCanvasPage";

const searchParams = z.object({
  version: z.coerce.bigint().positive().optional().catch(undefined),
});

export const Route = createFileRoute("/pipelines/$id/canvas")({
  validateSearch: searchParams,
  component: PipelineCanvasPage,
});
