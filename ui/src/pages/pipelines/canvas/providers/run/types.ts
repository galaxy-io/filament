import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

export interface PipelineCanvasRunState {
  runBindings: RunBinding[];
  isActivityOpen: boolean;
}
