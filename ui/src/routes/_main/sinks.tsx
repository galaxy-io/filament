import { create } from "@bufbuild/protobuf";
import { createFileRoute } from "@tanstack/react-router";
import z from "zod";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import ConnectionsPage from "@/pages/connectors/ConnectionsPage";

import { createListConnectionsQueryOptions } from "@/api/queries/connections";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const searchParams = z.object({
  q: z.string().optional().catch(undefined),
});

export const Route = createFileRoute("/_main/sinks")({
  validateSearch: searchParams,
  loader: () =>
    queryClient.ensureQueryData(
      createListConnectionsQueryOptions({
        input: create(ListConnectionsRequestSchema, { kind: ConnectorKind.SINK }),
        transport,
      }),
    ),
  component: () => <ConnectionsPage kind={ConnectorKind.SINK} />,
});
