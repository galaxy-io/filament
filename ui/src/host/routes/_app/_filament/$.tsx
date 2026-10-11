import { createFileRoute } from "@tanstack/react-router";

import NotFoundPage from "@/pages/NotFoundPage";

import { filamentNotFoundRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/$")({
  ...filamentNotFoundRouteOptions,
  component: NotFoundPage,
});
