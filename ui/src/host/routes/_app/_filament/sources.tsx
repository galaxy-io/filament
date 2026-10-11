import { createFileRoute } from "@tanstack/react-router";

import SourcesPage from "@/pages/connections/SourcesPage";

import { sourcesRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/sources")({
  ...sourcesRouteOptions,
  component: SourcesPage,
});
