import { ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ConnectorMaturity } from "@/gen/ingestion/v1/providers_pb";

export const CONNECTOR_GRID_MIN_COLUMN_WIDTH = 320;
export const CONNECTOR_DRAWER_WIDTH = 600;
export const CREATE_CONNECTION_MODAL_SELECTOR_WIDTH = 900;
export const CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH = 480;
export const CREATE_CONNECTION_MODAL_MIN_HEIGHT = 620;
export const CREATE_CONNECTION_MODAL_MAX_HEIGHT = 720;
export const CREATE_CONNECTION_SELECTOR_GHOST_COUNT = 6;
export const CREATE_CONNECTION_SELECTOR_GRID_COLUMNS = "repeat(3, 1fr)";
export const CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT = 150;

export const CONNECTOR_KIND_TO_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "—",
  [ConnectorKind.SOURCE]: "Source",
  [ConnectorKind.SINK]: "Sink",
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

export const CONNECTOR_MATURITY_TO_LABEL_MAP: Record<ConnectorMaturity, string> = {
  [ConnectorMaturity.UNSPECIFIED]: "—",
  [ConnectorMaturity.ALPHA]: "Alpha",
  [ConnectorMaturity.BETA]: "Beta",
  [ConnectorMaturity.STABLE]: "Stable",
};

export const CONNECTOR_MATURITY_TO_CHIP_VARIANT_MAP: Record<ConnectorMaturity, ChipVariant> = {
  [ConnectorMaturity.UNSPECIFIED]: ChipVariant.TERTIARY,
  [ConnectorMaturity.ALPHA]: ChipVariant.ORANGE,
  [ConnectorMaturity.BETA]: ChipVariant.BLUE,
  [ConnectorMaturity.STABLE]: ChipVariant.SUCCESS,
};
