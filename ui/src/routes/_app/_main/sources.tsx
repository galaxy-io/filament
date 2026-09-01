import { createFileRoute } from "@tanstack/react-router";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectionsPage from "@/pages/connectors/ConnectionsPage";

import {
  createListConnectionsInfiniteQueryOptions,
  createListConnectionsInput,
} from "@/api/queries/connections";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";
import { listSearchParamsSchema } from "@/api/utils";

const searchParams = listSearchParamsSchema.pick({ q: true });

export const Route = createFileRoute("/_app/_main/sources")({
  validateSearch: searchParams,
  loaderDeps: ({ search: { q } }) => ({ q }),
  loader: ({ deps }) =>
    queryClient.ensureInfiniteQueryData(
      createListConnectionsInfiniteQueryOptions({
        input: createListConnectionsInput(ConnectorKind.SOURCE, deps),
        transport,
      }),
    ),
  component: () => <ConnectionsPage kind={ConnectorKind.SOURCE} />,
});
