import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { usePipelineCanvasActiveRuns } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasActiveRuns";

export const usePipelineCanvasIsConnectionRunning = (connectionId: Connection["id"]) =>
  usePipelineCanvasActiveRuns().some(
    (run) => run.sourceConnectionId === connectionId || run.sinkConnectionId === connectionId,
  );
