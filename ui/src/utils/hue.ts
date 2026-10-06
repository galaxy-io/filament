import { ChartPalette } from "@galaxy-io/dls/charts/types";
import { SquareVariant } from "@galaxy-io/dls/shapes/Square";
import type { PaletteColor, StatusColor } from "@galaxy-io/dls/theme/tokens/types";

export type Hue = StatusColor | PaletteColor;

export type HueMark<Variant> = { variant: Variant } | { color: PaletteColor };

const STATUS_COLOR_TO_SQUARE_VARIANT_MAP: Record<StatusColor, SquareVariant> = {
  success: SquareVariant.SUCCESS,
  warning: SquareVariant.WARNING,
  error: SquareVariant.ERROR,
};

export const HUE_TO_CHART_PALETTE_MAP: Partial<Record<Hue, ChartPalette>> = {
  purple: ChartPalette.PURPLE,
  pink: ChartPalette.PINK,
  blue: ChartPalette.BLUE,
  teal: ChartPalette.TEAL,
  lime: ChartPalette.LIME,
  orange: ChartPalette.ORANGE,
  yellow: ChartPalette.YELLOW,
  green: ChartPalette.GREEN,
  success: ChartPalette.SUCCESS,
  warning: ChartPalette.WARNING,
  error: ChartPalette.ERROR,
};

const isStatusColor = (hue: Hue): hue is StatusColor => hue in STATUS_COLOR_TO_SQUARE_VARIANT_MAP;

export const hueToSquareMark = (hue: Hue | null): HueMark<SquareVariant> => {
  if (hue === null) return { variant: SquareVariant.TERTIARY };
  if (isStatusColor(hue)) return { variant: STATUS_COLOR_TO_SQUARE_VARIANT_MAP[hue] };
  return { color: hue };
};

export const hueToChartPalette = (hue: Hue | null): ChartPalette | undefined =>
  hue === null ? undefined : HUE_TO_CHART_PALETTE_MAP[hue];
