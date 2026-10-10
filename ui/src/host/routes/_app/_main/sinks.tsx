import { createFileRoute } from "@tanstack/react-router";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectionsPage from "@/pages/connectors/ConnectionsPage";

import { listSearchParamsSchema } from "@/api/utils";

const searchParams = listSearchParamsSchema.pick({ q: true });

export const Route = createFileRoute("/_app/_main/sinks")({
  validateSearch: searchParams,
  remountDeps: ({ search: { q } }) => ({ q }),
  component: () => <ConnectionsPage kind={ConnectorKind.SINK} />,
});
