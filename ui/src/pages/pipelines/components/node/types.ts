import type { JsonValue } from "@bufbuild/protobuf";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

export type PipelineNodeConfig = Record<string, JsonValue>;

export interface PipelineNodeConfigState {
  scopedFields: ConfigField[];
  fields: ConfigField[];
  displayValue: PipelineNodeConfig;
}
