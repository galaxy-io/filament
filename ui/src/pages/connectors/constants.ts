import { ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

// Layout constants
export const CONNECTOR_SEARCH_WIDTH = 280;
export const CONNECTOR_KIND_FILTER_WIDTH = 140;
export const CONNECTOR_GRID_MIN_COLUMN_WIDTH = 320;
export const CONNECTOR_DRAWER_WIDTH = 600;
export const CREATE_CONNECTION_MODAL_WIDTH = 720;
export const CREATE_CONNECTION_MODAL_SELECTOR_MIN_HEIGHT = 620;
export const CREATE_CONNECTION_MODAL_MAX_HEIGHT = 720;

// ConnectorKind → Label (singular)
export const CONNECTOR_KIND_TO_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: "—",
  [ConnectorKind.SOURCE]: "Source",
  [ConnectorKind.SINK]: "Sink",
};

// ConnectorKind → Label (plural, for filters)
export const CONNECTOR_KIND_FILTER_TO_LABEL_MAP: Record<ConnectorKind, string> =
  {
    [ConnectorKind.UNSPECIFIED]: "All connectors",
    [ConnectorKind.SOURCE]: "Sources",
    [ConnectorKind.SINK]: "Sinks",
  };

// ConnectorKind → Description
export const CONNECTOR_KIND_TO_DESCRIPTION_MAP: Record<ConnectorKind, string> =
  {
    [ConnectorKind.UNSPECIFIED]: "All connector types",
    [ConnectorKind.SOURCE]: "Ingest data from external systems",
    [ConnectorKind.SINK]: "Send data to external destinations",
  };

// ConnectorKind → Chip variant
export const CONNECTOR_KIND_TO_CHIP_VARIANT_MAP: Record<
  ConnectorKind,
  ChipVariant
> = {
  [ConnectorKind.UNSPECIFIED]: ChipVariant.TERTIARY,
  [ConnectorKind.SOURCE]: ChipVariant.LIME,
  [ConnectorKind.SINK]: ChipVariant.PINK,
};

export const SELECT_INPUT_OPTIONS_CONNECTOR_KIND: SelectInputOption[] = [
  {
    id: String(ConnectorKind.UNSPECIFIED),
    label: CONNECTOR_KIND_FILTER_TO_LABEL_MAP[ConnectorKind.UNSPECIFIED],
    value: ConnectorKind.UNSPECIFIED,
  },
  {
    id: String(ConnectorKind.SOURCE),
    label: CONNECTOR_KIND_FILTER_TO_LABEL_MAP[ConnectorKind.SOURCE],
    value: ConnectorKind.SOURCE,
  },
  {
    id: String(ConnectorKind.SINK),
    label: CONNECTOR_KIND_FILTER_TO_LABEL_MAP[ConnectorKind.SINK],
    value: ConnectorKind.SINK,
  },
];

export const ACRONYMS_TO_CAPITALIZE: string[] = [
  "api",
  "url",
  "id",
  "s3",
  "aws",
  "sql",
  "json",
  "http",
  "ssh",
  "ssl",
  "dsn",
];
