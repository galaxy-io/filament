import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import PipelineCanvasPage from "@/pages/pipelines/PipelineCanvasPage";

const searchParams = z.object({
  node: z.string().optional().catch(undefined),
  resource: z.string().optional().catch(undefined),
  showPanel: z.boolean().optional().catch(undefined),
  tab: z.enum(PipelineCanvasPanelTab).optional().catch(undefined),
});

export const Route = createFileRoute("/pipelines/$id/canvas")({
  validateSearch: searchParams,
  component: PipelineCanvasPage,
});
