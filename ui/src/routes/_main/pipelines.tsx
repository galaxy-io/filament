import { createFileRoute } from "@tanstack/react-router";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

import { createListPipelinesQueryOptions } from "@/api/queries/pipelines";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

export const Route = createFileRoute("/_main/pipelines")({
  loader: () =>
    queryClient.ensureQueryData(createListPipelinesQueryOptions({ transport })),
  component: PipelinesPage,
});
