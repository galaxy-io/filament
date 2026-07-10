import { ProviderKind } from "@/gen/ingestion/v1/common_pb";

// Layout constants
export const PROVIDER_SEARCH_WIDTH = 280;
export const PROVIDER_GRID_MIN_COLUMN_WIDTH = 320;
export const PROVIDER_DRAWER_WIDTH = 600;

// Maps
export const PROVIDER_KIND_TO_LABEL_MAP: Record<ProviderKind, string> = {
  [ProviderKind.UNSPECIFIED]: "All providers",
  [ProviderKind.SOURCE]: "Sources",
  [ProviderKind.SINK]: "Sinks",
};
