import { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import { SkeletonSize } from "@galaxy-io/dls/feedback/Skeleton";
import { TextSize } from "@galaxy-io/dls/text/Text";
import type { PaletteColor } from "@galaxy-io/dls/theme/tokens/types";
import { EMPTY_VALUE } from "@galaxy-io/dls/utils/format";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { ConnectorTileSize } from "@/components/connections/types";

import { FilamentPath } from "@/module/paths";

export const CONNECTOR_KIND_TO_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: EMPTY_VALUE,
  [ConnectorKind.SOURCE]: "Source",
  [ConnectorKind.SINK]: "Sink",
};

export const CONNECTOR_KIND_TO_PLURAL_LABEL_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: EMPTY_VALUE,
  [ConnectorKind.SOURCE]: "Sources",
  [ConnectorKind.SINK]: "Sinks",
};

export const CONNECTOR_KIND_TO_NOUN_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: EMPTY_VALUE,
  [ConnectorKind.SOURCE]: "source",
  [ConnectorKind.SINK]: "sink",
};

export const CONNECTOR_KIND_TO_PLURAL_NOUN_MAP: Record<ConnectorKind, string> = {
  [ConnectorKind.UNSPECIFIED]: EMPTY_VALUE,
  [ConnectorKind.SOURCE]: "sources",
  [ConnectorKind.SINK]: "sinks",
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

export const CONNECTOR_TILE_SIZE_TO_SIZE_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 20,
  [ConnectorTileSize.MEDIUM]: 24,
  [ConnectorTileSize.LARGE]: 36,
};

export const CONNECTOR_TILE_SIZE_TO_SKELETON_SIZE_MAP: Record<ConnectorTileSize, SkeletonSize> = {
  [ConnectorTileSize.SMALL]: SkeletonSize.X_SMALL,
  [ConnectorTileSize.MEDIUM]: SkeletonSize.SMALL,
  [ConnectorTileSize.LARGE]: SkeletonSize.LARGE,
};

export const CONNECTOR_TILE_SIZE_TO_LOGO_HEIGHT_MAP: Record<ConnectorTileSize, number> = {
  [ConnectorTileSize.SMALL]: 14,
  [ConnectorTileSize.MEDIUM]: 16,
  [ConnectorTileSize.LARGE]: 24,
};

export const CONNECTOR_TILE_SIZE_TO_TEXT_SIZE_MAP: Record<ConnectorTileSize, TextSize> = {
  [ConnectorTileSize.SMALL]: TextSize.CAPTION,
  [ConnectorTileSize.MEDIUM]: TextSize.BODY_MD,
  [ConnectorTileSize.LARGE]: TextSize.BODY_LG,
};
