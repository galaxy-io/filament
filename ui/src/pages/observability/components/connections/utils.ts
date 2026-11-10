import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { mapVersionNodesToFlowEndpoints } from "@/components/pipelines/utils";

import type { ObservabilityConnectionStats } from "@/pages/observability/components/connections/types";
import {
  type ObservabilityPipelineEntry,
  ObservabilityPipelineHealth,
} from "@/pages/observability/types";

export const createObservabilityConnectionStats = (
  entries: ObservabilityPipelineEntry[],
): Map<Connection["id"], ObservabilityConnectionStats> => {
  const stats = new Map<Connection["id"], ObservabilityConnectionStats>();
  for (const { pipeline, health } of entries) {
    const { sourceId, sinkIds } = mapVersionNodesToFlowEndpoints(
      pipeline.currentVersion?.graph?.nodes ?? [],
    );
    for (const connectionId of new Set([sourceId, ...sinkIds])) {
      if (!connectionId) {
        continue;
      }
      const current = stats.get(connectionId) ?? { pipelineCount: 0, isLive: false };
      stats.set(connectionId, {
        pipelineCount: current.pipelineCount + 1,
        isLive: current.isLive || health === ObservabilityPipelineHealth.RUNNING,
      });
    }
  }
  return stats;
};
