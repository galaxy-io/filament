import { ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export const CONNECTOR_GRID_MIN_COLUMN_WIDTH = 320;
export const CONNECTOR_DRAWER_WIDTH = 600;
export const CREATE_CONNECTION_MODAL_SELECTOR_WIDTH = 900;
export const CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH = 480;
export const CREATE_CONNECTION_MODAL_MIN_HEIGHT = 620;
export const CREATE_CONNECTION_MODAL_MAX_HEIGHT = 720;

export const CONNECTOR_KIND_TO_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "—",
  [ConnectorKind.SOURCE]: "Source",
  [ConnectorKind.SINK]: "Sink",
};

export type ConnectorKindParam = "SOURCE" | "SINK";

export const CONNECTOR_KIND_TO_PARAM_MAP: Partial<Record<ConnectorKind, ConnectorKindParam>> = {
  [ConnectorKind.SOURCE]: "SOURCE",
  [ConnectorKind.SINK]: "SINK",
};

export const CONNECTOR_KIND_PARAM_TO_KIND_MAP: Record<ConnectorKindParam, ConnectorKind> = {
  SOURCE: ConnectorKind.SOURCE,
  SINK: ConnectorKind.SINK,
};

export const CONNECTOR_KIND_TO_DESCRIPTION_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "All connector types",
  [ConnectorKind.SOURCE]: "Ingest data from external systems",
  [ConnectorKind.SINK]: "Send data to external destinations",
};

export const CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP: Record<
  ConnectorKind.SOURCE | ConnectorKind.SINK,
  string
> = {
  [ConnectorKind.SOURCE]: "Connect data sources to move data in.",
  [ConnectorKind.SINK]: "Connect data sinks to move data out.",
};

export const CONNECTOR_KIND_TO_CHIP_VARIANT_MAP: Record<ConnectorKind, ChipVariant> = {
  [ConnectorKind.UNSPECIFIED]: ChipVariant.TERTIARY,
  [ConnectorKind.SOURCE]: ChipVariant.LIME,
  [ConnectorKind.SINK]: ChipVariant.PINK,
};
