import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

export enum PipelineResourceStatusField {
  READ_MODE = "readMode",
  CURSOR = "cursor",
  WRITE_MODE = "writeMode",
}

export interface PipelineResourceStatus {
  resource: Resource["name"];
  field: PipelineResourceStatusField;
  message: string;
  isBlocking: boolean;
}
