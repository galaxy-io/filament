import { createFileRoute } from "@tanstack/react-router";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectionsPage from "@/pages/connectors/ConnectionsPage";

export const Route = createFileRoute("/_main/sources")({
  component: () => <ConnectionsPage kind={ConnectorKind.SOURCE} />,
});
