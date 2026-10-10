export enum Flow {
  CREATE_CONNECTION = "CREATE_CONNECTION",
  EDIT_CONNECTION = "EDIT_CONNECTION",
  CREATE_PIPELINE = "CREATE_PIPELINE",
}

declare module "@tanstack/react-router" {
  interface StaticDataRouteOption {
    filament?: boolean;
  }
}

export enum FilamentNavItem {
  OBSERVABILITY = "OBSERVABILITY",
  PIPELINES = "PIPELINES",
  SOURCES = "SOURCES",
  SINKS = "SINKS",
}
