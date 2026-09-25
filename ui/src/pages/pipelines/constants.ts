import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

export const PIPELINE_EXECUTION_MODES: ExecutionMode[] = [
  ExecutionMode.BOUNDED,
  ExecutionMode.CONTINUOUS,
];
