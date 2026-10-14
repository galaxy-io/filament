import { createFileRoute } from "@tanstack/react-router";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

import { pipelinesRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/pipelines/")({
  ...pipelinesRouteOptions,
  component: PipelinesPage,
});
