import type { FilamentFlowSeed, FilamentLayoutSearch } from "@/module/schemas";
import { type Flow, GalaxyProduct } from "@/module/types";

export type { GalaxyProductUrls } from "@/module/types";

export { GalaxyProduct };

export enum FilamentPath {
  OBSERVABILITY = "./observability",
  PIPELINES = "./pipelines",
  PIPELINE = "./pipelines/$id",
  PIPELINE_CANVAS = "./pipelines/$id/canvas",
  PIPELINE_HISTORY = "./pipelines/$id/history",
  PIPELINE_SETTINGS = "./pipelines/$id/settings",
  SOURCES = "./sources",
  SINKS = "./sinks",
  SETTINGS_TEAM = "./settings/team",
  SETTINGS_SERVICE_ACCOUNTS = "./settings/service-accounts",
}

export const createFilamentHref = (path: FilamentPath, params: Record<string, string> = {}) =>
  path.replace(/\$(\w+)/g, (_, key: string) => encodeURIComponent(params[key] ?? ""));

const FILAMENT_FLOW_SEARCH_RESET: { [K in keyof Required<FilamentFlowSeed>]: undefined } = {
  connector: undefined,
  connectorKind: undefined,
  connectorSearch: undefined,
  pipelineId: undefined,
  sourceId: undefined,
  sinkIds: undefined,
  resources: undefined,
  resyncResources: undefined,
  returnFlow: undefined,
};

export const createFilamentFlowSearch = (
  flow?: Flow,
  seed: FilamentFlowSeed = {},
): Omit<FilamentLayoutSearch, "connectionId"> => ({
  ...FILAMENT_FLOW_SEARCH_RESET,
  ...seed,
  flow,
});
