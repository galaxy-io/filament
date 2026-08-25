export enum PipelineSidebarItem {
  CANVAS = "canvas",
  HISTORY = "history",
  SETTINGS = "settings",
}

export interface PipelineLayoutNavbarRunButtonState {
  /** Worker-configuration editor text for the run-override dropdown. */
  workerConfiguration: string;
}
