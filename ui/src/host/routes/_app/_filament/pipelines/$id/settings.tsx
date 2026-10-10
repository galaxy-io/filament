import { createFileRoute } from "@tanstack/react-router";

import PipelineSettingsPage from "@/pages/pipelines/PipelineSettingsPage";

import { pipelineSettingsRouteOptions } from "@/module/routes";

export const Route = createFileRoute("/_app/_filament/pipelines/$id/settings")({
  ...pipelineSettingsRouteOptions,
  component: PipelineSettingsPage,
});
