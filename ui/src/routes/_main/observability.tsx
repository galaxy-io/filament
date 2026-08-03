import { createFileRoute } from "@tanstack/react-router";

import ObservabilityPage from "@/pages/observability/ObservabilityPage";

export const Route = createFileRoute("/_main/observability")({
  component: ObservabilityPage,
});
