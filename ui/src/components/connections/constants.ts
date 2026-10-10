import { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import type { PaletteColor } from "@galaxy-io/dls/theme/tokens/types";
import { EMPTY_VALUE } from "@galaxy-io/dls/utils/format";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { FilamentPath } from "@/module/paths";

export const CONNECTOR_KIND_TO_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: EMPTY_VALUE,
  [ConnectorKind.SOURCE]: "Source",
  [ConnectorKind.SINK]: "Sink",
};

export const CONNECTOR_KIND_TO_CHIP_COLOR_MAP: Record<
  ConnectorKind,
  { variant: ChipVariant } | { color: PaletteColor }
> = {
  [ConnectorKind.UNSPECIFIED]: { variant: ChipVariant.TERTIARY },
  [ConnectorKind.SOURCE]: { color: "lime" },
  [ConnectorKind.SINK]: { color: "pink" },
};

export const CONNECTOR_KIND_TO_PATH_MAP: Record<ConnectorKind, FilamentPath> = {
  [ConnectorKind.UNSPECIFIED]: FilamentPath.SOURCES,
  [ConnectorKind.SOURCE]: FilamentPath.SOURCES,
  [ConnectorKind.SINK]: FilamentPath.SINKS,
};
