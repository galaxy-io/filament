import { createFileRoute } from "@tanstack/react-router";

import ConnectionsPage from "@/pages/connectors/ConnectionsPage";

export const Route = createFileRoute("/_main/connections")({
  component: ConnectionsPage,
});
