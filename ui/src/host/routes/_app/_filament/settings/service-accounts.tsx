import { createFileRoute } from "@tanstack/react-router";

import ServiceAccountsPage from "@/pages/settings/ServiceAccountsPage";

export const Route = createFileRoute("/_app/_filament/settings/service-accounts")({
  component: ServiceAccountsPage,
});
