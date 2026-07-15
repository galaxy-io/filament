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

export enum PipelineFlowSize {
  SMALL = "small",
  MEDIUM = "medium",
}

export interface PipelineListItem {
  id: string;
  name: string;
  health: PipelineHealth;
  source: string;
  sinks: string[];
  lastRunLabel: string;
  volumeLabel: string;
  scheduleLabel: string;
  isEnabled: boolean;
}
