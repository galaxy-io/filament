import { TextSize } from "@galaxy-io/dls/text/Text";

import { BaseHeaderSize } from "@/layouts/components/types";

export const BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP: Record<BaseHeaderSize, TextSize> = {
  [BaseHeaderSize.SMALL]: TextSize.BODY_MD,
  [BaseHeaderSize.MEDIUM]: TextSize.BODY_LG,
  [BaseHeaderSize.LARGE]: TextSize.HEADING_SM,
};

export const BASE_HEADER_SIZE_TO_ICON_SIZE_MAP: Record<BaseHeaderSize, number> = {
  [BaseHeaderSize.SMALL]: 14,
  [BaseHeaderSize.MEDIUM]: 16,
  [BaseHeaderSize.LARGE]: 20,
};

export const BASE_HEADER_SIZE_TO_GAP_MAP: Record<BaseHeaderSize, number> = {
  [BaseHeaderSize.SMALL]: 8,
  [BaseHeaderSize.MEDIUM]: 12,
  [BaseHeaderSize.LARGE]: 16,
};
