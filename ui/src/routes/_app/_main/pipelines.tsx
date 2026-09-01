import { createFileRoute } from "@tanstack/react-router";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

import { listSearchParamsSchema } from "@/api/utils";

export const Route = createFileRoute("/_app/_main/pipelines")({
  validateSearch: listSearchParamsSchema,
  remountDeps: ({ search: { q, sortBy, sortOrder } }) => ({ q, sortBy, sortOrder }),
  component: PipelinesPage,
});
