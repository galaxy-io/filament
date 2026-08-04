import { IngestionType } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

export const PIPELINE_CANVAS_INGESTION_TYPES: IngestionType[] = [
  IngestionType.SNAPSHOT_REPLACE,
  IngestionType.SNAPSHOT_UPSERT,
  IngestionType.APPEND,
  IngestionType.UPSERT,
  IngestionType.DELETE,
  IngestionType.CDC,
];

const specAllowsIngestionType = (spec: ConnectorSpec | undefined, type: IngestionType): boolean =>
  !spec || spec.supportedIngestionTypes.length === 0 || spec.supportedIngestionTypes.includes(type);

export const getSupportedIngestionTypes = (
  sourceSpec: ConnectorSpec | undefined,
  sinkSpec: ConnectorSpec | undefined,
): IngestionType[] =>
  PIPELINE_CANVAS_INGESTION_TYPES.filter(
    (type) => specAllowsIngestionType(sourceSpec, type) && specAllowsIngestionType(sinkSpec, type),
  );
