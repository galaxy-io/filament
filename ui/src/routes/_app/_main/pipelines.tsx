import { createFileRoute } from "@tanstack/react-router";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

import {
  createListPipelinesInfiniteQueryOptions,
  createListPipelinesInput,
} from "@/api/queries/pipelines";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";
import { listSearchParamsSchema } from "@/api/utils";

export const Route = createFileRoute("/_app/_main/pipelines")({
  validateSearch: listSearchParamsSchema,
  loaderDeps: ({ search: { q, sortBy, sortOrder } }) => ({ q, sortBy, sortOrder }),
  loader: ({ deps }) =>
    queryClient.ensureInfiniteQueryData(
      createListPipelinesInfiniteQueryOptions({
        input: createListPipelinesInput(deps),
        transport,
      }),
    ),
  component: PipelinesPage,
});
