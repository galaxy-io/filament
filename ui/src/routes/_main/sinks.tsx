import { createFileRoute } from "@tanstack/react-router";

import ConnectionsPage from "@/pages/connectors/ConnectionsPage";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export const Route = createFileRoute("/_main/sinks")({
  component: () => <ConnectionsPage kind={ConnectorKind.SINK} />,
});
