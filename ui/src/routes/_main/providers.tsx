import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";

import ProvidersPage from "@/pages/providers/ProvidersPage";

const providersSearchSchema = z.object({
  providerId: z.string().optional(),
});

export const Route = createFileRoute("/_main/providers")({
  component: ProvidersPage,
  validateSearch: providersSearchSchema,
});
