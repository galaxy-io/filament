import { createFileRoute } from "@tanstack/react-router";

import ProvidersPage from "@/pages/providers/ProvidersPage";

export const Route = createFileRoute("/_main/providers")({
  component: ProvidersPage,
});
