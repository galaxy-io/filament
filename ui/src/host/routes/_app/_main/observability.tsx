import { createFileRoute } from "@tanstack/react-router";

import ObservabilityPage from "@/pages/observability/ObservabilityPage";

import { observabilitySearchSchema } from "@/module/schemas";

export const Route = createFileRoute("/_app/_main/observability")({
  validateSearch: observabilitySearchSchema,
  component: ObservabilityPage,
});
