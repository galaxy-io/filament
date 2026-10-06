import { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import type { PaletteColor } from "@galaxy-io/dls/theme/tokens/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ConnectorMaturity } from "@/gen/ingestion/v1/connectors_pb";

import { CreateConnectionSelectorShelf } from "@/pages/connectors/components/create/types";

export const CONNECTOR_GRID_MIN_COLUMN_WIDTH = 320;
export const CREATE_CONNECTION_SELECTOR_GHOST_COUNT = 6;
export const CREATE_CONNECTION_SELECTOR_SIDEBAR_WIDTH = 240;
export const CREATE_CONNECTION_SELECTOR_GRID_COLUMNS = "repeat(3, minmax(0, 1fr))";

export const CREATE_CONNECTION_SELECTOR_SHELF_ORDER: CreateConnectionSelectorShelf[] = [
  CreateConnectionSelectorShelf.ALL,
  CreateConnectionSelectorShelf.STABLE,
  CreateConnectionSelectorShelf.BETA,
  CreateConnectionSelectorShelf.ALPHA,
];

export const CREATE_CONNECTION_SELECTOR_SHELF_TO_LABEL_MAP: Record<
  CreateConnectionSelectorShelf,
  string
> = {
  [CreateConnectionSelectorShelf.ALL]: "All",
  [CreateConnectionSelectorShelf.STABLE]: "Stable",
  [CreateConnectionSelectorShelf.BETA]: "Beta",
  [CreateConnectionSelectorShelf.ALPHA]: "Alpha",
};
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

export const CONNECTOR_KIND_TO_CREATE_TITLE_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "New connection",
  [ConnectorKind.SOURCE]: "New source",
  [ConnectorKind.SINK]: "New sink",
};

export const CONNECTOR_KIND_TO_DOCS_PATH_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "/pages/connectors/overview/introduction",
  [ConnectorKind.SOURCE]: "/pages/connectors/overview/introduction#sources",
  [ConnectorKind.SINK]: "/pages/connectors/overview/introduction#sinks",
};

export const CONNECTOR_KIND_TO_EMPTY_MESSAGE_MAP: Record<
  ConnectorKind.SOURCE | ConnectorKind.SINK,
  string
> = {
  [ConnectorKind.SOURCE]: "Connect data sources to move data in.",
  [ConnectorKind.SINK]: "Connect data sinks to move data out.",
};

export const CONNECTOR_KIND_TO_CHIP_COLOR_MAP: Record<
  ConnectorKind,
  { variant: ChipVariant } | { color: PaletteColor }
> = {
  [ConnectorKind.UNSPECIFIED]: { variant: ChipVariant.TERTIARY },
  [ConnectorKind.SOURCE]: { color: "lime" },
  [ConnectorKind.SINK]: { color: "pink" },
};

export const CONNECTOR_MATURITY_TO_STATUS_MAP: Record<ConnectorMaturity, string | undefined> = {
  [ConnectorMaturity.UNSPECIFIED]: undefined,
  [ConnectorMaturity.ALPHA]: "experimental",
  [ConnectorMaturity.BETA]: "in beta",
  [ConnectorMaturity.STABLE]: undefined,
};
