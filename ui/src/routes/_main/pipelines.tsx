import { createFileRoute } from "@tanstack/react-router";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

export const Route = createFileRoute("/_main/pipelines")({
  component: PipelinesPage,
});
