export enum PipelineStatus {
  DRAFT = "draft",
  ACTIVE = "active",
  PAUSED = "paused",
}

export enum PipelineSidebarItem {
  CANVAS = "canvas",
  HISTORY = "history",
  SETTINGS = "settings",
}

export interface PipelineResource {
  id: string;
  name: string;
  status: PipelineStatus;
  source?: string;
  sinks?: string[];
}
