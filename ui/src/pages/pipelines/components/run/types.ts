import type { FilamentFlowSeed } from "@/module/schemas";

export enum PipelineRunResourceMode {
  CONTINUE = "CONTINUE",
  RESYNC = "RESYNC",
}

export interface PipelineRunModalState {
  workerConfiguration: string;
  selectedResources: string[];
  resyncResources: string[];
  isResourcesPaneOpen: boolean;
}

export type PipelineRunModalSeed = Pick<FilamentFlowSeed, "resources" | "resyncResources">;
