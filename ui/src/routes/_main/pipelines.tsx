import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

import { createListPipelinesInfiniteQueryOptions } from "@/api/queries/pipelines";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const searchParams = z.object({
  q: z.string().optional().catch(undefined),
});

export const Route = createFileRoute("/_main/pipelines")({
  validateSearch: searchParams,
  loader: () =>
    queryClient.ensureInfiniteQueryData(createListPipelinesInfiniteQueryOptions({ transport })),
  component: PipelinesPage,
});
