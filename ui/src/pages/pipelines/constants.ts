import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { getEnumValues } from "@/utils/select";

export const PIPELINE_EXECUTION_MODES = getEnumValues(ExecutionMode);

export const PIPELINES_DOCS_PATH = "/pages/guides/usage/web#pipelines";
