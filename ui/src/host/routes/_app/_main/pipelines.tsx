import { createFileRoute } from "@tanstack/react-router";

import PipelinesPage from "@/pages/pipelines/PipelinesPage";

import { pipelinesSearchSchema } from "@/module/schemas";

export const Route = createFileRoute("/_app/_main/pipelines")({
  validateSearch: pipelinesSearchSchema,
  remountDeps: ({ search: { q, sortBy, sortOrder } }) => ({ q, sortBy, sortOrder }),
  component: PipelinesPage,
});
