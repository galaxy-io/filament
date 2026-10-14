import { createFileRoute } from "@tanstack/react-router";

import SinksPage from "@/pages/connections/SinksPage";

import { sinksRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/sinks")({
  ...sinksRouteOptions,
  component: SinksPage,
});
