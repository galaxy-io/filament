import { createFileRoute } from "@tanstack/react-router";

import ConnectorsPage from "@/pages/connectors/ConnectorsPage";

export const Route = createFileRoute("/_main/connectors")({
  component: ConnectorsPage,
});
