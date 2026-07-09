export enum PipelineGroup {
  ACTIVE = "ACTIVE",
  NEEDS_ATTENTION = "NEEDS_ATTENTION",
  PAUSED = "PAUSED",
}

export enum PipelineHealth {
  HEALTHY = "HEALTHY",
  DEGRADED = "DEGRADED",
  FAILING = "FAILING",
  PAUSED = "PAUSED",
}

export interface PipelineListItem {
  id: string;
  name: string;
  health: PipelineHealth;
  /** Provider name of the source node, e.g. "postgres". */
  source: string;
  /** Provider names of the sink nodes, in display order. */
  sinks: string[];
  /** Preformatted last-run label, e.g. "14:29:44" or "now"; "—" when unknown. */
  lastRunLabel: string;
  /** Preformatted volume label, e.g. "1.2M rows"; "—" when unknown. */
  volumeLabel: string;
  /** Preformatted schedule label, e.g. "HOURLY", "15M"; "—" when unknown. */
  scheduleLabel: string;
  isEnabled: boolean;
}
