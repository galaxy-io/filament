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
