import { TextSize } from "@galaxy-io/dls/text/Text";

import { LayoutSize } from "@/layouts/types";

export const LAYOUT_SIZE_TO_GAP_MAP: Record<LayoutSize, number> = {
  [LayoutSize.SMALL]: 12,
  [LayoutSize.MEDIUM]: 20,
  [LayoutSize.LARGE]: 24,
};

export const LAYOUT_SIZE_TO_GLYPH_SIZE_MAP: Record<LayoutSize, number> = {
  [LayoutSize.SMALL]: 20,
  [LayoutSize.MEDIUM]: 24,
  [LayoutSize.LARGE]: 32,
};

export const LAYOUT_SIZE_TO_HEADER_SIZE_MAP: Record<LayoutSize, TextSize> = {
  [LayoutSize.SMALL]: TextSize.BODY_MD,
  [LayoutSize.MEDIUM]: TextSize.BODY_LG,
  [LayoutSize.LARGE]: TextSize.HEADING_SM,
};

export const LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP: Record<LayoutSize, TextSize> = {
  [LayoutSize.SMALL]: TextSize.BODY_SM,
  [LayoutSize.MEDIUM]: TextSize.BODY_MD,
  [LayoutSize.LARGE]: TextSize.BODY_LG,
};
