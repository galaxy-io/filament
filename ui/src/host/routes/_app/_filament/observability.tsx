import { createFileRoute } from "@tanstack/react-router";

import ObservabilityPage from "@/pages/observability/ObservabilityPage";

import { observabilityRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/observability")({
  ...observabilityRouteOptions,
  component: ObservabilityPage,
});
