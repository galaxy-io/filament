import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { ObservabilityTimeframe } from "@/pages/observability/types";

interface ObservabilityMetrics {
  totalRecords: bigint;
  totalBytes: bigint;
  statusCounts: Partial<Record<RunStatus, number>>;
}

export const OBSERVABILITY_TIMEFRAME_TO_METRICS_MAP: Record<
  ObservabilityTimeframe,
  ObservabilityMetrics
> = {
  [ObservabilityTimeframe.TWENTY_FOUR_HOURS]: {
    totalRecords: 1_284_503n,
    totalBytes: 4_512_887_296n,
    statusCounts: {
      [RunStatus.COMPLETED]: 312,
      [RunStatus.FAILED]: 47,
      [RunStatus.RUNNING]: 6,
      [RunStatus.REQUESTED]: 12,
      [RunStatus.CANCELED]: 4,
      [RunStatus.PAUSED]: 2,
      [RunStatus.PARTIAL]: 9,
    },
  },
  [ObservabilityTimeframe.SEVEN_DAYS]: {
    totalRecords: 8_965_311n,
    totalBytes: 31_138_512_896n,
    statusCounts: {
      [RunStatus.COMPLETED]: 2184,
      [RunStatus.FAILED]: 316,
      [RunStatus.RUNNING]: 6,
      [RunStatus.REQUESTED]: 41,
      [RunStatus.CANCELED]: 28,
      [RunStatus.PAUSED]: 5,
      [RunStatus.PARTIAL]: 64,
    },
  },
  [ObservabilityTimeframe.THIRTY_DAYS]: {
    totalRecords: 38_412_776n,
    totalBytes: 133_465_178_112n,
    statusCounts: {
      [RunStatus.COMPLETED]: 9360,
      [RunStatus.FAILED]: 1352,
      [RunStatus.RUNNING]: 6,
      [RunStatus.REQUESTED]: 176,
      [RunStatus.CANCELED]: 119,
      [RunStatus.PAUSED]: 12,
      [RunStatus.PARTIAL]: 274,
    },
  },
};
