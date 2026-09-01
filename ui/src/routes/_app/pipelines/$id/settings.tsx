import { createFileRoute } from "@tanstack/react-router";

import PipelineSettingsPage from "@/pages/pipelines/PipelineSettingsPage";

export const Route = createFileRoute("/_app/pipelines/$id/settings")({
  component: PipelineSettingsPage,
});
