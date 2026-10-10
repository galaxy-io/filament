import { createFileRoute } from "@tanstack/react-router";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectionsPage from "@/pages/connections/ConnectionsPage";

import { connectionsSearchSchema } from "@/module/schemas";

export const Route = createFileRoute("/_app/_main/sources")({
  validateSearch: connectionsSearchSchema,
  remountDeps: ({ search: { q } }) => ({ q }),
  component: () => <ConnectionsPage kind={ConnectorKind.SOURCE} />,
});
