import { create } from "@bufbuild/protobuf";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { ListPipelinesRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

export const OBSERVABILITY_SETUP_CONNECTIONS_INPUT = create(ListConnectionsRequestSchema, {
  includeDeleted: true,
});

export const OBSERVABILITY_PIPELINES_INPUT = create(ListPipelinesRequestSchema, {
  includeDeleted: true,
});
