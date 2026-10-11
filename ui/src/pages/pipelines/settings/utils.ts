import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

export const getPipelineSourceConnectionIds = (
  pipeline: Pipeline | undefined,
): Connection["id"][] => [
  ...new Set(
    (pipeline?.currentVersion?.graph?.nodes ?? [])
      .filter((node) => node.kind === ConnectorKind.SOURCE)
      .map((node) => node.connectionId),
  ),
];
