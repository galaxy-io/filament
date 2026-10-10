export enum Flow {
  CREATE_CONNECTION = "CREATE_CONNECTION",
  EDIT_CONNECTION = "EDIT_CONNECTION",
  CREATE_PIPELINE = "CREATE_PIPELINE",
  SETTINGS = "SETTINGS",
}

declare module "@tanstack/react-router" {
  interface StaticDataRouteOption {
    filament?: boolean;
  }
}
