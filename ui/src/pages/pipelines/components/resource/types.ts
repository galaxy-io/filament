import type { JsonValue } from "@bufbuild/protobuf";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumns } from "@/gen/ingestion/v1/connectors_pb";

import type { CanvasNode } from "@/pages/pipelines/canvas/types";

export enum PipelineResourceStatusField {
  READ_MODE = "readMode",
  CURSOR = "cursor",
  WRITE_MODE = "writeMode",
  COLUMNS = "columns",
  PARTITIONS = "partitions",
}

export interface PipelineResourcePartitionRow {
  name: ResourceColumns["partitions"][number];
}

export interface PipelineResourceStatus {
  resource: Resource["name"];
  field: PipelineResourceStatusField;
  message: string;
  isBlocking: boolean;
}

export interface PipelineResourceCreateState {
  resource: Resource["name"];
  sinkId: CanvasNode["id"];
}

export type PipelineResourceSinkConfig = Record<ConfigField["name"], JsonValue>;

export interface PipelineResourceSinkFieldsState {
  fields: ConfigField[];
  pipelineValues: PipelineResourceSinkConfig;
}
